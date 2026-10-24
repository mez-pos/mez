package winsvc

import (
	"strings"
	"testing"
)

func TestWithStartByUsers(t *testing.T) {
	const windowsDefault = "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)S:(AU;FA;CCDCLCSWRPWPDTLOCRSDRCWDWO;;WD)"
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "the right goes at the end of the discretionary list, before the audit list",
			in:   windowsDefault,
			want: "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)" + startByUsersACE + "S:(AU;FA;CCDCLCSWRPWPDTLOCRSDRCWDWO;;WD)",
		},
		{
			name: "with no audit list it goes at the end",
			in:   "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)",
			want: "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)" + startByUsersACE,
		},
		{
			name: "sc sdshow's blank lines around it are dropped",
			in:   "\r\n" + windowsDefault + "\r\n",
			want: withStartByUsers(windowsDefault),
		},
		{
			name: "a descriptor that grants it already is left alone",
			in:   withStartByUsers(windowsDefault),
			want: withStartByUsers(windowsDefault),
		},
		{
			name: "an empty descriptor becomes one list with the right",
			in:   "",
			want: "D:" + startByUsersACE,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withStartByUsers(tt.in)
			if got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
			if strings.Count(got, startByUsersACE) != 1 {
				t.Errorf("the right appears %d times", strings.Count(got, startByUsersACE))
			}
		})
	}
}
