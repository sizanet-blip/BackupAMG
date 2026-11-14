package system

import "testing"

func TestJoinRepoFollowsTheNewLetter(t *testing.T) {
	id := `\\?\Volume{12345678-1234-1234-1234-123456789abc}\Copias`
	cases := []struct {
		stored, mount, want string
	}{
		{`E:\Copias`, `F:\`, `F:\Copias`},
		{`E:\Copias\`, `F:\`, `F:\Copias`},
		{`E:\`, `F:\`, `F:\`},
		{id, `G:\`, `G:\Copias`},
		{`\\nas\copias`, `F:\`, `\\nas\copias`},
	}
	for _, tc := range cases {
		if got := JoinRepo(tc.stored, tc.mount); got != tc.want {
			t.Fatalf("%s en %s: %s, se esperaba %s", tc.stored, tc.mount, got, tc.want)
		}
	}
}
