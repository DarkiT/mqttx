package mqttx

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"
)

// 证书格式类型
const (
	CertFormatPEM = "pem" // PEM格式证书
	CertFormatDER = "der" // DER格式证书
	CertFormatP12 = "p12" // PKCS#12格式证书
	CertFormatJKS = "jks" // Java KeyStore格式
	CertFormatPFX = "pfx" // PFX格式证书（PKCS#12的别名）
)

// SecurityError 安全相关错误
type SecurityError struct {
	Message string
	Err     error
}

func (e *SecurityError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *SecurityError) Unwrap() error {
	return e.Err
}

// newSecurityError 创建安全相关错误
func newSecurityError(message string, err error) *SecurityError {
	return &SecurityError{
		Message: message,
		Err:     err,
	}
}

// EnhancedTLSConfig 增强的TLS配置
type EnhancedTLSConfig struct {
	// 基本TLS配置
	CAFile     string // CA证书文件路径
	CertFile   string // 客户端证书文件路径
	KeyFile    string // 客户端密钥文件路径
	SkipVerify bool   // 是否跳过服务器证书验证

	// 增强配置
	CAFormat         string        // CA证书格式
	CertFormat       string        // 客户端证书格式
	KeyFormat        string        // 客户端密钥格式
	KeyPassword      string        // 私钥密码
	CertPassword     string        // 证书密码
	MinVersion       uint16        // 最低TLS版本
	MaxVersion       uint16        // 最高TLS版本
	CipherSuites     []uint16      // 加密套件列表
	CurvePreferences []tls.CurveID // 曲线偏好

	// 证书轮换配置
	AutoReload        bool          // 是否自动重新加载证书
	ReloadInterval    time.Duration // 重新加载间隔
	CertExpireWarning time.Duration // 证书过期警告时间

	// 运行时状态
	certExpiry time.Time    // 证书过期时间
	lastReload time.Time    // 上次重新加载时间
	certMutex  sync.RWMutex // 证书访问互斥锁
	tlsConfig  *tls.Config  // 缓存的TLS配置
	watcher    *CertWatcher // 证书监视器
}

// CertWatcher 证书监视器
// 用于监控证书文件变化和过期时间
type CertWatcher struct {
	config     *EnhancedTLSConfig
	stopCh     chan struct{}
	logger     Logger
	reloadFunc func() error
	wg         sync.WaitGroup
}

// NewEnhancedTLSConfig 创建增强的TLS配置
func NewEnhancedTLSConfig() *EnhancedTLSConfig {
	return &EnhancedTLSConfig{
		CAFormat:          CertFormatPEM,
		CertFormat:        CertFormatPEM,
		KeyFormat:         CertFormatPEM,
		MinVersion:        tls.VersionTLS12,
		AutoReload:        false,
		ReloadInterval:    24 * time.Hour,
		CertExpireWarning: 30 * 24 * time.Hour, // 30天
	}
}

// WithCACert 设置CA证书
func (c *EnhancedTLSConfig) WithCACert(caFile, format string) *EnhancedTLSConfig {
	c.CAFile = caFile
	if format != "" {
		c.CAFormat = format
	}
	return c
}

// WithClientCert 设置客户端证书
func (c *EnhancedTLSConfig) WithClientCert(certFile, keyFile, certFormat, keyFormat string) *EnhancedTLSConfig {
	c.CertFile = certFile
	c.KeyFile = keyFile
	if certFormat != "" {
		c.CertFormat = certFormat
	}
	if keyFormat != "" {
		c.KeyFormat = keyFormat
	}
	return c
}

// WithPassword 设置证书和私钥密码
func (c *EnhancedTLSConfig) WithPassword(certPassword, keyPassword string) *EnhancedTLSConfig {
	c.CertPassword = certPassword
	c.KeyPassword = keyPassword
	return c
}

// WithTLSVersion 设置TLS版本范围
func (c *EnhancedTLSConfig) WithTLSVersion(minVersion, maxVersion uint16) *EnhancedTLSConfig {
	c.MinVersion = minVersion
	if maxVersion > 0 {
		c.MaxVersion = maxVersion
	}
	return c
}

// WithCipherSuites 设置加密套件
func (c *EnhancedTLSConfig) WithCipherSuites(cipherSuites []uint16) *EnhancedTLSConfig {
	c.CipherSuites = cipherSuites
	return c
}

// WithCurvePreferences 设置曲线偏好
func (c *EnhancedTLSConfig) WithCurvePreferences(curves []tls.CurveID) *EnhancedTLSConfig {
	c.CurvePreferences = curves
	return c
}

// WithAutoReload 设置自动重新加载
func (c *EnhancedTLSConfig) WithAutoReload(autoReload bool, reloadInterval time.Duration) *EnhancedTLSConfig {
	c.AutoReload = autoReload
	if reloadInterval > 0 {
		c.ReloadInterval = reloadInterval
	}
	return c
}

// WithCertExpireWarning 设置证书过期警告时间
func (c *EnhancedTLSConfig) WithCertExpireWarning(warning time.Duration) *EnhancedTLSConfig {
	c.CertExpireWarning = warning
	return c
}

// BuildTLSConfig 构建TLS配置
func (c *EnhancedTLSConfig) BuildTLSConfig() (*tls.Config, error) {
	c.certMutex.Lock()
	defer c.certMutex.Unlock()

	// 如果已经有缓存的配置且未启用自动重新加载，则直接返回
	if c.tlsConfig != nil && !c.AutoReload {
		return c.tlsConfig, nil
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.SkipVerify,
		MinVersion:         c.MinVersion,
	}

	if c.MaxVersion > 0 {
		tlsConfig.MaxVersion = c.MaxVersion
	}

	if len(c.CipherSuites) > 0 {
		tlsConfig.CipherSuites = c.CipherSuites
	}

	if len(c.CurvePreferences) > 0 {
		tlsConfig.CurvePreferences = c.CurvePreferences
	}

	// 加载CA证书
	if c.CAFile != "" {
		caCertPool, err := c.loadCACert()
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = caCertPool
	}

	// 加载客户端证书
	if c.CertFile != "" && c.KeyFile != "" {
		cert, err := c.loadClientCert()
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}

		// 获取证书过期时间
		if len(cert.Certificate) > 0 {
			x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
			if err == nil {
				c.certExpiry = x509Cert.NotAfter
			}
		}
	}

	c.tlsConfig = tlsConfig
	c.lastReload = time.Now()

	return tlsConfig, nil
}

// loadCACert 加载CA证书
func (c *EnhancedTLSConfig) loadCACert() (*x509.CertPool, error) {
	caCertPool := x509.NewCertPool()

	switch c.CAFormat {
	case CertFormatPEM:
		caCert, err := ioutil.ReadFile(c.CAFile)
		if err != nil {
			return nil, newSecurityError("failed to read CA file", err)
		}
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			return nil, newSecurityError("failed to parse CA certificate", nil)
		}
	case CertFormatDER:
		caCert, err := ioutil.ReadFile(c.CAFile)
		if err != nil {
			return nil, newSecurityError("failed to read CA file", err)
		}
		cert, err := x509.ParseCertificate(caCert)
		if err != nil {
			return nil, newSecurityError("failed to parse DER CA certificate", err)
		}
		caCertPool.AddCert(cert)
	default:
		return nil, newSecurityError(fmt.Sprintf("unsupported CA certificate format: %s", c.CAFormat), nil)
	}

	return caCertPool, nil
}

// loadClientCert 加载客户端证书
func (c *EnhancedTLSConfig) loadClientCert() (tls.Certificate, error) {
	var cert tls.Certificate
	var err error

	// 根据证书格式加载
	switch c.CertFormat {
	case CertFormatPEM:
		if c.KeyPassword != "" {
			// 如果有密钥密码，使用自定义加载方法
			cert, err = loadPEMWithPassword(c.CertFile, c.KeyFile, c.KeyPassword)
		} else {
			// 否则使用标准方法
			cert, err = tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		}
		if err != nil {
			return cert, newSecurityError("failed to load client certificate", err)
		}
	default:
		return cert, newSecurityError(fmt.Sprintf("unsupported certificate format: %s", c.CertFormat), nil)
	}

	return cert, nil
}

// loadPEMWithPassword 加载带密码的PEM格式证书
func loadPEMWithPassword(certFile, keyFile, password string) (tls.Certificate, error) {
	// 读取证书文件
	certPEMBlock, err := ioutil.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, err
	}

	// 读取私钥文件
	keyPEMBlock, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, err
	}

	// 解析私钥
	var keyDERBlock *pem.Block
	for {
		keyDERBlock, keyPEMBlock = pem.Decode(keyPEMBlock)
		if keyDERBlock == nil {
			return tls.Certificate{}, errors.New("failed to parse key PEM data")
		}
		if keyDERBlock.Type == "PRIVATE KEY" || keyDERBlock.Type == "RSA PRIVATE KEY" {
			break
		}
	}

	// 如果私钥被加密，使用密码解密
	if x509.IsEncryptedPEMBlock(keyDERBlock) {
		derBytes, err := x509.DecryptPEMBlock(keyDERBlock, []byte(password))
		if err != nil {
			return tls.Certificate{}, errors.New("failed to decrypt key")
		}

		// 根据私钥类型重新编码
		var pemType string
		if keyDERBlock.Type == "RSA PRIVATE KEY" {
			pemType = "RSA PRIVATE KEY"
		} else {
			pemType = "PRIVATE KEY"
		}

		keyDERBlock = &pem.Block{
			Type:  pemType,
			Bytes: derBytes,
		}
	}

	// 重新编码私钥
	keyPEMBlock = pem.EncodeToMemory(keyDERBlock)

	// 使用标准方法加载证书和私钥
	return tls.X509KeyPair(certPEMBlock, keyPEMBlock)
}

// StartCertWatcher 启动证书监视器
func (c *EnhancedTLSConfig) StartCertWatcher(logger Logger, reloadFunc func() error) error {
	if !c.AutoReload {
		return nil
	}

	if c.watcher != nil {
		return newSecurityError("certificate watcher already started", nil)
	}

	c.watcher = &CertWatcher{
		config:     c,
		stopCh:     make(chan struct{}),
		logger:     logger,
		reloadFunc: reloadFunc,
	}

	c.watcher.start()
	return nil
}

// StopCertWatcher 停止证书监视器
func (c *EnhancedTLSConfig) StopCertWatcher() {
	if c.watcher != nil {
		c.watcher.stop()
		c.watcher = nil
	}
}

// GetCertExpiry 获取证书过期时间
func (c *EnhancedTLSConfig) GetCertExpiry() time.Time {
	c.certMutex.RLock()
	defer c.certMutex.RUnlock()
	return c.certExpiry
}

// IsExpiringSoon 检查证书是否即将过期
func (c *EnhancedTLSConfig) IsExpiringSoon() bool {
	c.certMutex.RLock()
	defer c.certMutex.RUnlock()

	if c.certExpiry.IsZero() {
		return false
	}

	return time.Now().Add(c.CertExpireWarning).After(c.certExpiry)
}

// ReloadCertificates 重新加载证书
func (c *EnhancedTLSConfig) ReloadCertificates() error {
	// 清除缓存的TLS配置，强制重新加载
	c.certMutex.Lock()
	c.tlsConfig = nil
	c.certMutex.Unlock()

	// 重新构建TLS配置
	_, err := c.BuildTLSConfig()
	return err
}

// start 启动证书监视器
func (w *CertWatcher) start() {
	w.wg.Add(1)
	go w.watchCertificates()
}

// stop 停止证书监视器
func (w *CertWatcher) stop() {
	close(w.stopCh)
	w.wg.Wait()
}

// watchCertificates 监视证书文件变化和过期时间
func (w *CertWatcher) watchCertificates() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.ReloadInterval)
	defer ticker.Stop()

	// 获取初始文件信息
	caInfo := w.getFileInfo(w.config.CAFile)
	certInfo := w.getFileInfo(w.config.CertFile)
	keyInfo := w.getFileInfo(w.config.KeyFile)

	for {
		select {
		case <-ticker.C:
			// 检查证书是否即将过期
			if w.config.IsExpiringSoon() {
				w.logger.Warn("Certificate is expiring soon",
					"expiry", w.config.GetCertExpiry(),
					"warning_period", w.config.CertExpireWarning)
			}

			// 检查文件是否已更改
			newCAInfo := w.getFileInfo(w.config.CAFile)
			newCertInfo := w.getFileInfo(w.config.CertFile)
			newKeyInfo := w.getFileInfo(w.config.KeyFile)

			if w.fileChanged(caInfo, newCAInfo) ||
				w.fileChanged(certInfo, newCertInfo) ||
				w.fileChanged(keyInfo, newKeyInfo) {
				w.logger.Info("Certificate files changed, reloading")

				// 更新文件信息
				caInfo = newCAInfo
				certInfo = newCertInfo
				keyInfo = newKeyInfo

				// 重新加载证书
				if err := w.config.ReloadCertificates(); err != nil {
					w.logger.Error("Failed to reload certificates", "error", err)
				} else if w.reloadFunc != nil {
					if err := w.reloadFunc(); err != nil {
						w.logger.Error("Failed to apply reloaded certificates", "error", err)
					}
				}
			}
		case <-w.stopCh:
			return
		}
	}
}

// getFileInfo 获取文件信息
func (w *CertWatcher) getFileInfo(path string) os.FileInfo {
	if path == "" {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	return info
}

// fileChanged 检查文件是否已更改
func (w *CertWatcher) fileChanged(old, new os.FileInfo) bool {
	if old == nil && new == nil {
		return false
	}

	if old == nil || new == nil {
		return true
	}

	return old.ModTime() != new.ModTime() || old.Size() != new.Size()
}

// WithEnhancedTLS 使用增强的TLS配置
func (o *Options) WithEnhancedTLS(config *EnhancedTLSConfig) *Options {
	// 将增强的TLS配置转换为基本TLS配置
	o.TLS = &TLSConfig{
		CAFile:     config.CAFile,
		CertFile:   config.CertFile,
		KeyFile:    config.KeyFile,
		SkipVerify: config.SkipVerify,
	}

	// 存储增强的TLS配置供后续使用
	o.EnhancedTLS = config

	return o
}

// ConfigureTLSWithEnhanced 使用增强的TLS配置构建TLS配置
func (o *Options) ConfigureTLSWithEnhanced() (*tls.Config, error) {
	if o.EnhancedTLS != nil {
		return o.EnhancedTLS.BuildTLSConfig()
	}

	// 如果没有增强的TLS配置，则使用基本的TLS配置
	return o.ConfigureTLS()
}
