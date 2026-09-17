package stat

import (
	"strings"
	"testing"
)

func TestParseOptions(t *testing.T) {
	id1 := strings.Repeat("a", 64)
	id2 := strings.Repeat("b", 64)
	option, err := parseOptions([]string{"--container-id", id1 + ":123", "--container-id", id2 + ":456"})
	if err != nil {
		t.Fatal(err)
	}
	if len(option.containers) != 2 || option.containers[0] != (containerTarget{id: id1, pid: 123}) || option.containers[1] != (containerTarget{id: id2, pid: 456}) {
		t.Fatalf("unexpected parsed options: %#v", option.containers)
	}
}

func TestParseOptionsRejectsInvalidTargets(t *testing.T) {
	id := strings.Repeat("a", 64)
	tests := [][]string{
		{"--container-id"},
		{"--container-id", id},
		{"--container-id", strings.ToUpper(id) + ":123"},
		{"--container-id", id + ":1"},
		{"--container-id", id + ":0123"},
		{"--container-id", id + ":123:456"},
		{"--container-id", id + ":123", "--container-id", id + ":456"},
		{"--unknown", id + ":123"},
	}
	for _, args := range tests {
		if _, err := parseOptions(args); err == nil {
			t.Errorf("parseOptions(%q) unexpectedly succeeded", args)
		}
	}
}

func TestOuterContainerCgroup(t *testing.T) {
	id := strings.Repeat("a", 64)
	tests := []struct {
		name string
		path string
		want string
		ok   bool
	}{
		{name: "cgroupfs", path: "/docker/" + id + "/sysbox/inner", want: "/docker/" + id, ok: true},
		{name: "systemd", path: "/system.slice/docker-" + id + ".scope/sysbox/inner", want: "/system.slice/docker-" + id + ".scope", ok: true},
		{name: "containerd systemd", path: "/system.slice/cri-containerd-" + id + ".scope/inner", want: "/system.slice/cri-containerd-" + id + ".scope", ok: true},
		{name: "different container", path: "/docker/" + strings.Repeat("b", 64), ok: false},
		{name: "relative", path: "docker/" + id, ok: false},
		{name: "unclean", path: "/docker/../docker/" + id, ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := outerContainerCgroup(test.path, id)
			if got != test.want || ok != test.ok {
				t.Fatalf("outerContainerCgroup() = %q, %v; want %q, %v", got, ok, test.want, test.ok)
			}
		})
	}
}
