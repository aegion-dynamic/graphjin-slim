package serv_test

import (
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/serv/v3"
	"github.com/stretchr/testify/assert"
)

func TestNormalStart_NilDBInDevMode(t *testing.T) {
	conf, err := serv.NewConfig("app_name: test\n", "yaml")
	assert.NoError(t, err)
	s, err := serv.NewGraphJinService(conf)
	assert.NoError(t, err)
	assert.NotNil(t, s)
}

// Note: Testing production mode with nil DB would require mocking the database
// connection, which is complex. The key behavior difference is tested above:
// in dev mode, normalStart returns early with no error when DB is nil.
// In production mode, the function would proceed to NewGraphJin and fail.
