package server

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/registry/auth"
	"github.com/endophage/gotuf/signed"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"

	"github.com/docker/notary/server/handlers"
	"github.com/docker/notary/utils"
)

// Run sets up and starts a TLS server that can be cancelled using the
// given configuration. The context it is passed is the context it should
// use directly for the TLS server, and generate children off for requests
func Run(ctx context.Context, addr, tlsCertFile, tlsKeyFile string, trust signed.CryptoService, authMethod string, authOpts interface{}) error {

	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return err
	}
	var lsnr net.Listener
	lsnr, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}

	keypair, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
	if err != nil {
		// must be able to run without certs. In prod, users may
		// want load balancer to terminate TLS
		logrus.Errorf("[Notary Server] Error loading TLS keys %s", err)
	} else {

		tlsConfig := &tls.Config{
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
			Certificates: []tls.Certificate{keypair},
			Rand:         rand.Reader,
		}

		lsnr = tls.NewListener(lsnr, tlsConfig)
	}

	var ac auth.AccessController
	if authMethod == "" {
		ac = nil
	} else {
		authOptions, ok := authOpts.(map[string]interface{})
		if !ok {
			return fmt.Errorf("auth.options must be a map[string]interface{}")
		}
		ac, err = auth.GetAccessController(authMethod, authOptions)
		if err != nil {
			return err
		}
	}
	hand := utils.RootHandlerFactory(ac, ctx, trust)

	r := mux.NewRouter()
	// TODO (endophage): use correct regexes for image and tag names
	r.Methods("POST").Path("/v2/{imageName:.*}/_trust/tuf/").Handler(hand(handlers.AtomicUpdateHandler, "push", "pull"))
	r.Methods("GET").Path("/v2/{imageName:.*}/_trust/tuf/{tufRole:(root|targets|snapshot)}.json").Handler(hand(handlers.GetHandler, "pull"))
	r.Methods("GET").Path("/v2/{imageName:.*}/_trust/tuf/timestamp.json").Handler(hand(handlers.GetTimestampHandler, "pull"))
	r.Methods("GET").Path("/v2/{imageName:.*}/_trust/tuf/timestamp.key").Handler(hand(handlers.GetTimestampKeyHandler, "push", "pull"))
	r.Methods("POST").Path("/v2/{imageName:.*}/_trust/tuf/{tufRole:(root|targets|timestamp|snapshot)}.json").Handler(hand(handlers.UpdateHandler, "push", "pull"))
	r.Methods("DELETE").Path("/v2/{imageName:.*}/_trust/tuf/").Handler(hand(handlers.DeleteHandler, "push", "pull"))

	svr := http.Server{
		Addr:    addr,
		Handler: r,
	}

	logrus.Error("[Notary Server] : Starting on ", addr)

	err = svr.Serve(lsnr)

	return err
}
