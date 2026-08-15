package bootstrap

import (
	"crypto/tls"

	"github.com/go-sql-driver/mysql"
)

// init registers a custom TLS config named "aiven" with the go-sql-driver/mysql
// driver. Aiven MySQL requires SSL but uses a self-signed CA that is not in the
// system trust store, so we skip certificate verification (the connection is
// still encrypted). The DSN in config/database.go or entrypoint.sh should use
// tls=aiven instead of tls=true.
//
// This runs before any service provider boots, so the MySQL driver picks it up
// when the ORM initializes.
func init() {
	if err := mysql.RegisterTLSConfig("aiven", &tls.Config{
		InsecureSkipVerify: true,
	}); err != nil {
		panic("failed to register aiven TLS config: " + err.Error())
	}
}
