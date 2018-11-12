package cmd

import (
	"context"
	"path/filepath"

	"github.com/minio/cli"

	"github.com/pydio/minio-srv/cmd/logger"
)

func StartPydioGateway(ctx context.Context, gw Gateway, gatewayAddr string, configDir string, certFile string, certKey string) {

	// Disallow relative paths, figure out absolute paths.
	configDirAbs, err := filepath.Abs(configDir)
	logger.FatalIf(err, "Unable to fetch absolute path for config directory %s", configDir)

	setConfigDir(configDirAbs)

	cliContext := &cli.Context{}
	cliContext.Set("address", gatewayAddr)

	StartGateway(cliContext, gw)

	/*
		// Initialize gateway config.
		initConfig()

		// Enable loggers as per configuration file.
		logger.EnableQuiet()
		enableLoggers()

		// Init the error tracing module.
		initError()

		// Check and load SSL certificates.
		globalPublicCerts, globalRootCAs, globalTLSCerts, globalIsSSL, err = getSSLConfig()
		fatalIf(err, "Invalid SSL certificate file")

		if certFile != "" && certKey != "" {
			var cert tls.Certificate
			cert, err = tls.LoadX509KeyPair(certFile, certKey)
			fatalIf(err, "Cannot load SSL certificate files")
			globalTLSCertificate = &cert
			globalIsSSL = true
		}

		initNSLock(false) // Enable local namespace lock.

		newObject, err := newPydioGateway()
		// if err != nil {
		// 	return err
		// }

		router := mux.NewRouter().SkipClean(true)

		registerGatewayPydioAPIRouter(router, newObject)

		var handlerFns = []HandlerFunc{
			// Validate all the incoming paths.
			setPathValidityHandler,
			// Limits all requests size to a maximum fixed limit
			setRequestSizeLimitHandler,
			// Adds 'crossdomain.xml' policy handler to serve legacy flash clients.
			setCrossDomainPolicy,
			// Validates all incoming requests to have a valid date header.
			// Redirect some pre-defined browser request paths to a static location prefix.
			setBrowserRedirectHandler,
			// Validates if incoming request is for restricted buckets.
			setReservedBucketHandler,
			// Adds cache control for all browser requests.
			setBrowserCacheControlHandler,
			// Validates all incoming requests to have a valid date header.
			setTimeValidityHandler,
			// CORS setting for all browser API requests.
			setCorsHandler,
			// Validates all incoming URL resources, for invalid/unsupported
			// resources client receives a HTTP error.
			setIgnoreResourcesHandler,
			// Auth handler verifies incoming authorization headers and
			// routes them accordingly. Client receives a HTTP error for
			// invalid/unsupported signatures.
			setAuthHandler,
			// Add new handlers here.
			getPydioAuthHandlerFunc(true),
			// Add Span Handler
			servicecontext.HttpSpanHandlerWrapper,
		}

		globalHTTPServer = http.NewServer([]string{gatewayAddr}, registerHandlers(router, handlerFns...), globalTLSCertificate)

		// Start server, automatically configures TLS if certs are available.
		go func() {
			globalHTTPServerErrorCh <- globalHTTPServer.Start()
		}()

		signal.Notify(globalOSSignalCh, os.Interrupt, syscall.SIGTERM)

		// Once endpoints are finalized, initialize the new object api.
		globalObjLayerMutex.Lock()
		globalObjectAPI = newObject
		globalObjLayerMutex.Unlock()

		// Prints the formatted startup message once object layer is initialized.
		printGatewayStartupMessage(getAPIEndpoints(gatewayAddr), pydioBackend)
	*/

	stopProcess := func() bool {
		var err error
		logger.Info("Shutting down Minio Server")
		err = globalHTTPServer.Shutdown()
		logger.Info("Unable to shutdown http server " + err.Error())

		//oerr = newObject.Shutdown()
		//errorIf(oerr, "Unable to shutdown object layer")
		return true

	}

	select {
	case e := <-globalHTTPServerErrorCh:
		logger.Info("Minio Service: Received Error on globalHTTPServerErrorCh", e)
		stopProcess()
		return
	case <-globalOSSignalCh:
		logger.Info("Minio Service: Received globalOSSignalCh")
		stopProcess()
		return
	case <-ctx.Done():
		logger.Info("Minio Service: Received ctx.Done()")
		stopProcess()
		return
	}

	//handleSignals()
}
