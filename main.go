package main

import (
	"flag"
	"fmt"
	"github.com/cernbox/cboxauthd/handlers"
	"github.com/cernbox/cboxauthd/pkg/ldapuserbackend"
	gh "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"net/http"
	"os"
)

// Build information obtained with the help of -ldflags
var (
	appName       string
	buildDate     string // date -u
	gitTag        string // git describe --exact-match HEAD
	gitNearestTag string // git describe --abbrev=0 --tags HEAD
	gitCommit     string // git rev-parse HEAD
)

var fVersion bool
var logLevel = zapcore.InfoLevel

func init() {
	viper.SetDefault("port", 2020)
	viper.SetDefault("ldaphostname", "cerndc.cern.ch")
	viper.SetDefault("ldapport", 636)
	viper.SetDefault("ldapbindusername", "testuser")
	viper.SetDefault("ldapbindpassword", "testpassword")
	viper.SetDefault("ldapbasedn", "OU=Users,OU=Organic Units,DC=cern,DC=ch")
	viper.SetDefault("ldapfilter", "(samaccountname=%s)")
	viper.SetDefault("signingkey", "change me!!!")
	viper.SetDefault("applog", "stderr")
	viper.SetDefault("httplog", "stderr")
	viper.SetDefault("expiretime", 3600)
	viper.SetDefault("owncloudcookiename", "oc_sessionpassphrase")
	viper.SetDefault("loglevel", "info")

	viper.SetConfigName("cboxauthd")
	viper.AddConfigPath("/etc/cboxauthd/")

	flag.BoolVar(&fVersion, "version", false, "Show version")
	flag.Int("port", 2020, "Port to listen for connections")
	flag.String("ldaphostname", "cerndc.cern.ch", "Hostname of the LDAP server")
	flag.Int("ldapport", 636, "Port of LDAP server")
	flag.String("ldapbindusername", "CERN\\testuser", "The user to bind to LDAP")
	flag.String("ldapbindpassword", "testpassword", "The password to bind to LDAP")
	flag.String("ldapbasedn", "OU=Users,OU=Organic Units,DC=cern,DC=ch", "The base dn to use to talk to LDAP")
	flag.String("ldapfilter", "(samaccountname=%s)", "The filter to use in LDAP queries")
	flag.String("signingkey", "change me!!!", "The key to use to sign the JWT tokens")
	flag.String("applog", "stderr", "File to log application data")
	flag.String("httplog", "stderr", "File to log HTTP requests")
	flag.String("config", "", "Configuration file to use")
	flag.Int("expiretime", 3600, "Time in seconds the jwt/cookie will be valid")
	flag.String("owncloudcookiename", "oc_sessionpassphrase", "Cookie to store the auth session in the client")
	flag.String("loglevel", "info", "Level to log")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
}

func main() {

	if fVersion {
		showVersion()
	}

	if viper.GetString("config") != "" {
		viper.SetConfigFile(viper.GetString("config"))
	}

	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	
	err = logLevel.UnmarshalText([]byte(viper.GetString("loglevel")))
	if err != nil {
		panic(err)
	}
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(logLevel)
	config.OutputPaths = []string{viper.GetString("applog")}
	logger, err := config.Build()
	if err != nil {
		panic(err)
	}

	router := mux.NewRouter()

	ub := ldapuserbackend.New(viper.GetString("ldaphostname"), viper.GetInt("ldapport"), viper.GetString("ldapbasedn"), viper.GetString("ldapfilter"), viper.GetString("ldapbindusername"), viper.GetString("ldapbindpassword"))
	authHandler := handlers.CheckAuth(logger, ub, viper.GetString("signingkey"), viper.GetInt("expiretime"), viper.GetString("owncloudcookiename"))

	router.Handle("/api/v1/auth", authHandler).Methods("GET")

	out := getHTTPLoggerOut(viper.GetString("httplog"))
	loggedRouter := gh.LoggingHandler(out, router)

	logger.Info("server is listening", zap.Int("port", viper.GetInt("port")))
	logger.Warn("server stopped", zap.Error(http.ListenAndServe(fmt.Sprintf(":%d", viper.GetInt("port")), loggedRouter)))
}

func getHTTPLoggerOut(filename string) *os.File {
	if filename == "stderr" {
		return os.Stderr
	} else if filename == "stdout" {
		return os.Stdout
	} else {
		fd, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}
		return fd
	}
}

func showVersion() {
	// if gitTag is not empty we are on release build
	if gitTag != "" {
		fmt.Printf("%s %s commit:%s release-build\n", appName, gitNearestTag, gitCommit)
		os.Exit(0)
	}
	fmt.Printf("%s %s commit:%s dev-build\n", appName, gitNearestTag, gitCommit)
	os.Exit(0)
}