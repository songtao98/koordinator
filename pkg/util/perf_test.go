package util

import "testing"

func TestPerfCPUFlagToCPUs(t *testing.T) {
	tests := []struct {
		name   string
		flag   string
		exCpus []int
		errStr string
	}{
		{
			name:   "valid single CPU",
			flag:   "1",
			exCpus: []int{1},
		},
		{
			name:   "valid range CPUs",
			flag:   "1-5",
			exCpus: []int{1, 2, 3, 4, 5},
		},
		{
			name:   "valid double digit",
			flag:   "10",
			exCpus: []int{10},
		},
		{
			name:   "valid double digit range",
			flag:   "10-12",
			exCpus: []int{10, 11, 12},
		},
		{
			name:   "valid double digit stride",
			flag:   "10-20:5",
			exCpus: []int{10, 15, 20},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cpus, err := perfCPUFlagToCPUs(test.flag)
			if test.errStr != "" {
				if err != nil {
					t.Fatal("expected error to not be nil")
				}
				if test.errStr != err.Error() {
					t.Fatalf(
						"expected error %q, got %q",
						test.errStr,
						err.Error(),
					)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(cpus) != len(test.exCpus) {
				t.Fatalf(
					"expected CPUs %v, got %v",
					test.exCpus,
					cpus,
				)
			}
			for i := range cpus {
				if test.exCpus[i] != cpus[i] {
					t.Fatalf(
						"expected CPUs %v, got %v",
						test.exCpus[i],
						cpus[i],
					)
				}
			}
		})
	}
}
