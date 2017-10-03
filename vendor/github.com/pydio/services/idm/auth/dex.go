package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/coreos/dex/api"
	"github.com/coreos/dex/server"
	"github.com/coreos/dex/storage"
)

func serve(c Config) error {

	logger, err := newLogger(c.Logger.Level, c.Logger.Format)
	if err != nil {
		return fmt.Errorf("invalid config: %v", err)
	}
	if c.Logger.Level != "" {
		logger.Infof("config using log level: %s", c.Logger.Level)
	}

	// Fast checks. Perform these first for a more responsive CLI.
	checks := []struct {
		bad    bool
		errMsg string
	}{
		{c.Issuer == "", "no issuer specified in config file"},
		{!c.EnablePasswordDB && len(c.StaticPasswords) != 0, "cannot specify static passwords without enabling password db"},
		{c.Storage.Config == nil, "no storage suppied in config file"},
		{c.Web.HTTP == "" && c.Web.HTTPS == "", "must supply a HTTP/HTTPS  address to listen on"},
		{c.Web.HTTPS != "" && c.Web.TLSCert == "", "no cert specified for HTTPS"},
		{c.Web.HTTPS != "" && c.Web.TLSKey == "", "no private key specified for HTTPS"},
		{c.GRPC.TLSCert != "" && c.GRPC.Addr == "", "no address specified for gRPC"},
		{c.GRPC.TLSKey != "" && c.GRPC.Addr == "", "no address specified for gRPC"},
		{(c.GRPC.TLSCert == "") != (c.GRPC.TLSKey == ""), "must specific both a gRPC TLS cert and key"},
		{c.GRPC.TLSCert == "" && c.GRPC.TLSClientCA != "", "cannot specify gRPC TLS client CA without a gRPC TLS cert"},
	}

	for _, check := range checks {
		if check.bad {
			return fmt.Errorf("invalid config: %s", check.errMsg)
		}
	}

	logger.Infof("config issuer: %s", c.Issuer)

	var grpcOptions []grpc.ServerOption
	if c.GRPC.TLSCert != "" {
		if c.GRPC.TLSClientCA != "" {
			// Parse certificates from certificate file and key file for server.
			cert, err := tls.LoadX509KeyPair(c.GRPC.TLSCert, c.GRPC.TLSKey)
			if err != nil {
				return fmt.Errorf("invalid config: error parsing gRPC certificate file: %v", err)
			}

			// Parse certificates from client CA file to a new CertPool.
			cPool := x509.NewCertPool()
			clientCert, err := ioutil.ReadFile(c.GRPC.TLSClientCA)
			if err != nil {
				return fmt.Errorf("invalid config: reading from client CA file: %v", err)
			}
			if cPool.AppendCertsFromPEM(clientCert) != true {
				return errors.New("invalid config: failed to parse client CA")
			}

			tlsConfig := tls.Config{
				Certificates: []tls.Certificate{cert},
				ClientAuth:   tls.RequireAndVerifyClientCert,
				ClientCAs:    cPool,
			}
			grpcOptions = append(grpcOptions, grpc.Creds(credentials.NewTLS(&tlsConfig)))
		} else {
			opt, err := credentials.NewServerTLSFromFile(c.GRPC.TLSCert, c.GRPC.TLSKey)
			if err != nil {
				return fmt.Errorf("invalid config: load grpc certs: %v", err)
			}
			grpcOptions = append(grpcOptions, grpc.Creds(opt))
		}
	}

	s, err := c.Storage.Config.Open(logger)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %v", err)
	}
	logger.Infof("config storage: %s", c.Storage.Type)

	if len(c.StaticClients) > 0 {
		for _, client := range c.StaticClients {
			logger.Infof("config static client: %s", client.ID)
		}
		s = storage.WithStaticClients(s, c.StaticClients)
	}
	if len(c.StaticPasswords) > 0 {
		passwords := make([]storage.Password, len(c.StaticPasswords))
		for i, p := range c.StaticPasswords {
			passwords[i] = storage.Password(p)
		}
		s = storage.WithStaticPasswords(s, passwords, logger)
	}

	storageConnectors := make([]storage.Connector, len(c.StaticConnectors))
	for i, c := range c.StaticConnectors {
		if c.ID == "" || c.Name == "" || c.Type == "" {
			return fmt.Errorf("invalid config: ID, Type and Name fields are required for a connector")
		}
		if c.Config == nil {
			return fmt.Errorf("invalid config: no config field for connector %q", c.ID)
		}
		logger.Infof("config connector: %s", c.ID)

		// convert to a storage connector object
		conn, err := ToStorageConnector(c)
		if err != nil {
			return fmt.Errorf("failed to initialize storage connectors: %v", err)
		}
		storageConnectors[i] = conn

	}

	if c.EnablePasswordDB {
		storageConnectors = append(storageConnectors, storage.Connector{
			ID:   server.LocalConnector,
			Name: "Email",
			Type: server.LocalConnector,
		})
		logger.Infof("config connector: local passwords enabled")
	}

	s = storage.WithStaticConnectors(s, storageConnectors)

	if len(c.OAuth2.ResponseTypes) > 0 {
		logger.Infof("config response types accepted: %s", c.OAuth2.ResponseTypes)
	}
	if c.OAuth2.SkipApprovalScreen {
		logger.Infof("config skipping approval screen")
	}
	if len(c.Web.AllowedOrigins) > 0 {
		logger.Infof("config allowed origins: %s", c.Web.AllowedOrigins)
	}

	// explicitly convert to UTC.
	now := func() time.Time { return time.Now().UTC() }

	serverConfig := server.Config{
		SupportedResponseTypes: c.OAuth2.ResponseTypes,
		SkipApprovalScreen:     c.OAuth2.SkipApprovalScreen,
		AllowedOrigins:         c.Web.AllowedOrigins,
		Issuer:                 c.Issuer,
		Storage:                s,
		Web:                    c.Frontend,
		Logger:                 logger,
		Now:                    now,
	}
	if c.Expiry.SigningKeys != "" {
		signingKeys, err := time.ParseDuration(c.Expiry.SigningKeys)
		if err != nil {
			return fmt.Errorf("invalid config value %q for signing keys expiry: %v", c.Expiry.SigningKeys, err)
		}
		logger.Infof("config signing keys expire after: %v", signingKeys)
		serverConfig.RotateKeysAfter = signingKeys
	}
	if c.Expiry.IDTokens != "" {
		idTokens, err := time.ParseDuration(c.Expiry.IDTokens)
		if err != nil {
			return fmt.Errorf("invalid config value %q for id token expiry: %v", c.Expiry.IDTokens, err)
		}
		logger.Infof("config id tokens valid for: %v", idTokens)
		serverConfig.IDTokensValidFor = idTokens
	}

	serv, err := server.NewServer(context.Background(), serverConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize server: %v", err)
	}

	errc := make(chan error, 3)
	if c.Web.HTTP != "" {
		logger.Infof("listening (http) on %s", c.Web.HTTP)
		go func() {
			err := http.ListenAndServe(c.Web.HTTP, serv)
			errc <- fmt.Errorf("listening on %s failed: %v", c.Web.HTTP, err)
		}()
	}
	if c.Web.HTTPS != "" {
		logger.Infof("listening (https) on %s", c.Web.HTTPS)
		go func() {
			err := http.ListenAndServeTLS(c.Web.HTTPS, c.Web.TLSCert, c.Web.TLSKey, serv)
			errc <- fmt.Errorf("listening on %s failed: %v", c.Web.HTTPS, err)
		}()
	}
	if c.GRPC.Addr != "" {
		logger.Infof("listening (grpc) on %s", c.GRPC.Addr)
		go func() {
			errc <- func() error {
				list, err := net.Listen("tcp", c.GRPC.Addr)
				if err != nil {
					return fmt.Errorf("listening on %s failed: %v", c.GRPC.Addr, err)
				}
				s := grpc.NewServer(grpcOptions...)
				api.RegisterDexServer(s, server.NewAPI(serverConfig.Storage, logger))
				err = s.Serve(list)
				return fmt.Errorf("listening on %s failed: %v", c.GRPC.Addr, err)
			}()
		}()
	}

	return <-errc
}

var (
	logLevels  = []string{"debug", "info", "error"}
	logFormats = []string{"json", "text"}
)

type utcFormatter struct {
	f logrus.Formatter
}

func (f *utcFormatter) Format(e *logrus.Entry) ([]byte, error) {
	e.Time = e.Time.UTC()
	return f.f.Format(e)
}

func newLogger(level string, format string) (logrus.FieldLogger, error) {
	var logLevel logrus.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = logrus.DebugLevel
	case "", "info":
		logLevel = logrus.InfoLevel
	case "error":
		logLevel = logrus.ErrorLevel
	default:
		return nil, fmt.Errorf("log level is not one of the supported values (%s): %s", strings.Join(logLevels, ", "), level)
	}

	var formatter utcFormatter
	switch strings.ToLower(format) {
	case "", "text":
		formatter.f = &logrus.TextFormatter{DisableColors: true}
	case "json":
		formatter.f = &logrus.JSONFormatter{}
	default:
		return nil, fmt.Errorf("log format is not one of the supported values (%s): %s", strings.Join(logFormats, ", "), format)
	}

	return &logrus.Logger{
		Out:       os.Stderr,
		Formatter: &formatter,
		Level:     logLevel,
	}, nil
}
