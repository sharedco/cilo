@_default:
  just --list

test *args:
  (cd cilo && go test {{args}} ./...)

test-verbose:
  (cd cilo && go test -v ./...)

test-unit *args:
  (cd cilo && go test {{args}} ./...)

test-e2e:
  (cd cilo && CILO_E2E=1 go test -tags e2e ./tests/e2e)
