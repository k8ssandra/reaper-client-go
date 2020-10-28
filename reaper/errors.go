package reaper

import "errors"

var (
	CassandraClusterNotFound = errors.New("cassandra cluster not found")

	ErrRedirectsNotSupported = errors.New("http redirects are not supported")
)

