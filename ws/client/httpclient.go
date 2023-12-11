package client

import (
	"github.com/imroc/req/v3"
	"time"
)

var httpClient = req.NewClient().
	DisableAutoReadResponse().
	DisableCompression().
	DisableAutoDecode().
	EnableHTTP3().
	SetRedirectPolicy(req.NoRedirectPolicy()).
	SetCommonRetryCount(5).
	SetTimeout(60 * time.Second).
	SetCookieJar(nil)
