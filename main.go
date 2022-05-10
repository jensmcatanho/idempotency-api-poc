package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
)

var (
	db                           *sql.DB
	cache                        = memcache.New("memcached:11211")
	ErrEmptyIdempotencyKeyHeader = errors.New("empty Idempotency-Key header")
	ErrWrongCredentials          = errors.New("wrong credentials")
)

const (
	sqlHost     = "localhost"
	sqlPort     = 5432
	sqlUser     = "postgres"
	sqlPassword = "postgres"
	sqlDbName   = "idempotency"
)

type authenticateResponse struct {
	Token string
	Error error
}

type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func main() {
	fmt.Println("AAaaaaaaAAAAaAaaaAa")
	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
	e.POST("/authenticate", Authenticate)

	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", sqlUser, sqlPassword, sqlHost, sqlPort, sqlDbName)
	var err error
	db, err = sql.Open("postgres", url)
	if err != nil {
		panic(err)
	}

	if err := cache.Ping(); err != nil {
		panic(err)
	}

	if err := db.Ping(); err != nil {
		panic(err)
	}

	e.Logger.Fatal(e.Start(":8080"))
}

func Authenticate(echoContext echo.Context) error {
	idempotencyKey := echoContext.Request().Header.Get("Idempotency-Key")
	retryResponse := retryAuthenticate(idempotencyKey)
	if retryResponse.Error == nil {
		return echoContext.String(http.StatusOK, fmt.Sprintf("Already authenticated previously: %s", retryResponse.Token))
	}
	log.Printf("Retry failed with \"%s\" error. Performing a new authentication", retryResponse.Error)

	var credentials Credentials
	if err := (&echo.DefaultBinder{}).BindBody(echoContext, &credentials); err != nil {
		return err
	}

	authenticationResponse := doAuthentication(credentials)
	if authenticationResponse.Error != nil {
		return echoContext.NoContent(http.StatusUnauthorized)
	}

	if err := prepareRetryAuthenticate(idempotencyKey, authenticationResponse.Token); err != nil {
		log.Printf("Failed to prepare retry operation: %s", ErrEmptyIdempotencyKeyHeader)
	}

	return echoContext.String(http.StatusOK, fmt.Sprintf("Successfully authenticated: %s", authenticrationResponse.Token))
}

func doAuthentication(credentials Credentials) authenticateResponse {
	results, err := db.Query(fmt.Sprintf("SELECT * FROM users WHERE email=%s", credentials.Email))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return authenticateResponse{Error: ErrWrongCredentials}
		}

		return authenticateResponse{Error: err}
	}

	var user User
	for results.Next() {
		err := results.Scan(&user)
		if err != nil {
			return authenticateResponse{Error: err}
		}
	}

	if user.Password != credentials.Password {
		return authenticateResponse{Error: ErrWrongCredentials}
	}

	token := uuid.New()
	return authenticateResponse{Token: token.String()}
}

func retryAuthenticate(idempotencyKey string) authenticateResponse {
	if idempotencyKey == "" {
		token, err := cache.Get(idempotencyKey)
		if err == nil || !errors.Is(err, memcache.ErrCacheMiss) {
			if len(token.Value) > 0 {
				return authenticateResponse{Token: string(token.Value), Error: nil}
			}
		}
		return authenticateResponse{Error: err}
	}

	return authenticateResponse{Error: ErrEmptyIdempotencyKeyHeader}
}

func prepareRetryAuthenticate(idempotencyKey, token string) error {
	if idempotencyKey != "" {
		return cache.Set(&memcache.Item{Key: idempotencyKey, Value: []byte(token), Expiration: 10})
	}

	return ErrEmptyIdempotencyKeyHeader
}
