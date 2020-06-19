package main

import (
	"net/http"
	"net/url"
	"os"
	"time"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/fiatjaf/lnpay-go"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"
	"github.com/kelseyhightower/envconfig"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"gopkg.in/redis.v5"
)

var err error
var s Settings
var pg *sqlx.DB
var log = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stderr})
var lnp *lnpay.Client
var rds *redis.Client
var store *sessions.CookieStore
var lnpending *lnpay.Wallet
var lnpendingId string
var httpPublic = &assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, Prefix: ""}
var router = mux.NewRouter()

type Settings struct {
	Host               string `envconfig:"HOST" default:"0.0.0.0"`
	Port               string `envconfig:"PORT" required:"true"`
	SecretKey          string `envconfig:"SECRET_KEY" required:"true"`
	ServiceURL         string `envconfig:"SERVICE_URL" required:"true"`
	PostgresURL        string `envconfig:"POSTGRES_URL" required:"true"`
	RedisURL           string `envconfig:"REDIS_URL" required:"true"`
	LNPayKey           string `envconfig:"LNPAY_KEY" required:"true"`
	LNPayWalletPending string `envconfig:"LNPAY_WALLET_PENDING" required:"true"`
	LNPayWebhookSecret string `envconfig:"LNPAY_WEBHOOK_SECRET" required:"true"`
}

func main() {
	err = envconfig.Process("", &s)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't process envconfig.")
	}

	// cookie store
	store = sessions.NewCookieStore([]byte(s.SecretKey))

	// postgres connection
	pg, err = sqlx.Connect("postgres", s.PostgresURL)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't connect to postgres")
	}

	// lnpay client
	lnp = lnpay.NewClient(s.LNPayKey)
	lnpending = lnp.Wallet(s.LNPayWalletPending)
	details, err := lnpending.Details()
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't get lnpay wallet details")
	}
	lnpendingId = details.ID

	// redis connection
	rurl, _ := url.Parse(s.RedisURL)
	pw, _ := rurl.User.Password()
	rds = redis.NewClient(&redis.Options{
		Addr:     rurl.Host,
		Password: pw,
	})
	if err := rds.Ping().Err(); err != nil {
		log.Fatal().Err(err).Str("url", s.RedisURL).
			Msg("failed to connect to redis")
	}

	// routes
	router.PathPrefix("/static/").Methods("GET").Handler(http.FileServer(httpPublic))
	router.Path("/favicon.ico").Methods("GET").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			iconf, _ := httpPublic.Open("static/icon.png")
			fstat, _ := iconf.Stat()
			http.ServeContent(w, r, "static/icon.png", fstat.ModTime(), iconf)
			return
		})
	router.Path("/~/webhook/receive/" + s.LNPayWebhookSecret).Methods("POST").
		HandlerFunc(receivePaymentWebhook)
	router.Path("/~/auth").Methods("GET").HandlerFunc(authUser)
	router.Path("/~~~/auth").Methods("GET").HandlerFunc(authUserStream)
	router.Path("/~/list").Methods("GET").HandlerFunc(listFiles)
	router.Path("/~/add").Methods("POST").HandlerFunc(addFile)
	router.Path("/~/{file}").Methods("GET").HandlerFunc(buyFile)
	router.Path("/~~~/{file}").Methods("GET").HandlerFunc(buyFileStream)
	router.Path("/~/{key}/announce").Methods("GET").HandlerFunc(handleAnnounce)
	router.PathPrefix("/").Methods("GET").HandlerFunc(serveClient)

	// start http server
	log.Info().Str("host", s.Host).Str("port", s.Port).Msg("listening")
	srv := &http.Server{
		Handler:      router,
		Addr:         s.Host + ":" + s.Port,
		WriteTimeout: 300 * time.Second,
		ReadTimeout:  300 * time.Second,
	}
	err = srv.ListenAndServe()
	if err != nil {
		log.Error().Err(err).Msg("error serving http")
	}
}

func serveClient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	indexf, err := httpPublic.Open("static/index.html")
	if err != nil {
		log.Error().Err(err).Str("file", "static/index.html").
			Msg("make sure you generated bindata.go without -debug")
		return
	}
	fstat, _ := indexf.Stat()
	http.ServeContent(w, r, "static/index.html", fstat.ModTime(), indexf)
	return
}
