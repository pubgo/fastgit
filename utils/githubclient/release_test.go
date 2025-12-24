package githubclient

import (
	"context"
	"runtime"
	"testing"

	"github.com/pubgo/funk/v2/pretty"
	"github.com/samber/lo"
)

func TestName(t *testing.T) {
	rr := NewPublicRelease("pubgo", "fastcommit")
	ffff := lo.Must(rr.List(context.Background()))
	t.Log(runtime.GOARCH, runtime.GOOS)
	//pretty.Println(getAssets(ffff))
	pretty.Println(ffff)
}
