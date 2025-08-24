// Contributed by WINK Streaming (https://www.wink.co) - 2025
// MoQ protocol implementation for MediaMTX

package core

import (
	"github.com/bluenviron/mediamtx/internal/conf"
	"github.com/bluenviron/mediamtx/internal/logger"
	"github.com/bluenviron/mediamtx/internal/servers/moq"
)

// MoQServerManager manages the MoQ server lifecycle
type MoQServerManager struct {
	conf        *conf.Conf
	pathManager *pathManager
	parent      logger.Writer
	
	server *moq.Server
	enabled bool
}

// NewMoQServerManager creates a new MoQ server manager
func NewMoQServerManager(
	conf *conf.Conf,
	pathManager *pathManager,
	parent logger.Writer,
) *MoQServerManager {
	return &MoQServerManager{
		conf:        conf,
		pathManager: pathManager,
		parent:      parent,
		enabled:     false,
	}
}

// Initialize checks config and only creates server if enabled
func (m *MoQServerManager) Initialize() error {
	// Check if MoQ is enabled in config
	// For now, we'll enable it if address is set
	if m.conf.MoQAddress == "" {
		m.parent.Log(logger.Info, "MoQ support is disabled (no address configured)")
		return nil
	}
	
	// Only create the server if enabled
	m.server = &moq.Server{
		Address:     m.conf.MoQAddress,
		Encryption:  m.conf.MoQEncryption,
		ServerCert:  m.conf.MoQServerCert,
		ServerKey:   m.conf.MoQServerKey,
		AllowOrigin: m.conf.MoQAllowOrigin,
		ReadTimeout: m.conf.ReadTimeout,
		PathManager: m.pathManager,
		Parent:      m.parent,
	}
	
	err := m.server.Initialize()
	if err != nil {
		return err
	}
	
	m.enabled = true
	m.parent.Log(logger.Info, "MoQ server initialized successfully")
	return nil
}

// Close closes the MoQ server if it's running
func (m *MoQServerManager) Close() {
	if m.enabled && m.server != nil {
		m.server.Close()
		m.parent.Log(logger.Info, "MoQ server closed")
	}
}

// ReloadPathConfs reloads path configurations
func (m *MoQServerManager) ReloadPathConfs(pathConfs map[string]*conf.Path) {
	if m.enabled && m.server != nil {
		m.server.ReloadPathConfs(pathConfs)
	}
}

// PathReady is called when a path is ready
func (m *MoQServerManager) PathReady(pa *path) {
	if m.enabled && m.server != nil {
		// Convert internal path to MoQ path type
		moqPath := &moq.Path{}
		m.server.PathReady(moqPath)
	}
}

// PathNotReady is called when a path is not ready
func (m *MoQServerManager) PathNotReady(pa *path) {
	if m.enabled && m.server != nil {
		// Convert internal path to MoQ path type
		moqPath := &moq.Path{}
		m.server.PathNotReady(moqPath)
	}
}

// APIPathsList returns the list of paths
func (m *MoQServerManager) APIPathsList() (*moq.APIPathsList, error) {
	if m.enabled && m.server != nil {
		return m.server.APIPathsList()
	}
	return nil, nil
}