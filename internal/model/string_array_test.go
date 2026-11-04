package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringArrayValue(t *testing.T) {
	for _, tc := range []struct {
		in   StringArray
		want string
	}{
		{nil, "{}"},
		{StringArray{}, "{}"},
		{StringArray{"Đà Lạt"}, `{"Đà Lạt"}`},
		{StringArray{`a"b`, `c\d`, "x,y", ""}, `{"a\"b","c\\d","x,y",""}`},
	} {
		got, err := tc.in.Value()
		require.NoError(t, err)
		require.Equal(t, tc.want, got)
	}
}

func TestStringArrayScan(t *testing.T) {
	for _, tc := range []struct {
		in   any
		want StringArray
	}{
		{nil, StringArray{}},
		{"{}", StringArray{}},
		{[]byte("{abc}"), StringArray{"abc"}},
		{`{"Đà Lạt",công-tác}`, StringArray{"Đà Lạt", "công-tác"}},
		{`{"a\"b","c\\d","x,y",""}`, StringArray{`a"b`, `c\d`, "x,y", ""}},
		{`{a,NULL,"NULL"}`, StringArray{"a", "NULL"}},
		{[]string{"x"}, StringArray{"x"}},
	} {
		var got StringArray
		require.NoError(t, got.Scan(tc.in), "%v", tc.in)
		require.Equal(t, tc.want, got, "%v", tc.in)
	}

	var bad StringArray
	require.Error(t, bad.Scan("abc"))
	require.Error(t, bad.Scan(`{"abc}`))
	require.Error(t, bad.Scan(42))
}

func TestStringArrayRoundTrip(t *testing.T) {
	in := StringArray{"Du lịch", `quote " here`, `back\slash`, "{brace}", "  space  "}
	v, err := in.Value()
	require.NoError(t, err)
	var out StringArray
	require.NoError(t, out.Scan(v))
	require.Equal(t, in, out)
}
