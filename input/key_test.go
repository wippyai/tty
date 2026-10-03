// SPDX-License-Identifier: MPL-2.0

package input

import "testing"

func TestKeyCodeInvertsKeyName(t *testing.T) {
	for code, name := range keyNames {
		got, ok := KeyCode(name)
		if !ok || got != code {
			t.Errorf("KeyCode(%q) = %d, %v; want %d", name, got, ok, code)
		}
	}
	if _, ok := KeyCode("a"); ok {
		t.Error("KeyCode accepts a character")
	}
}
