package fuego

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePathParams(t *testing.T) {
	require.Equal(t, []string(nil), parseStdPathParams("/"))
	require.Equal(t, []string(nil), parseStdPathParams("/item/"))
	require.Equal(t, []string{"user"}, parseStdPathParams("POST /item/{user}"))
	require.Equal(t, []string{"user"}, parseStdPathParams("/item/{user}"))
	require.Equal(t, []string{"user", "id"}, parseStdPathParams("/item/{user}/{id}"))
	require.Equal(t, []string{"$"}, parseStdPathParams("/item/{$}"))
	require.Equal(t, []string{"user"}, parseStdPathParams("POST alt.com/item/{user}"))
}

func BenchmarkParsePathParams(b *testing.B) {
	b.Run("empty", func(b *testing.B) {
		for range b.N {
			parseStdPathParams("/")
		}
	})

	b.Run("several path params", func(b *testing.B) {
		for range b.N {
			parseStdPathParams("/item/{user}/{id}")
		}
	})
}

func FuzzParsePathParams(f *testing.F) {
	f.Add("/item/{user}")
	f.Add("/item/")
	f.Add("/item/{user}/{id}")
	f.Add("POST /item/{user}")
	f.Add("")

	f.Fuzz(func(t *testing.T, data string) {
		parseStdPathParams(data)
	})
}
