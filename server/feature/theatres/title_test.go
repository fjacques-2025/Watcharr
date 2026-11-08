package theatres

import "testing"

func TestIsUpper(t *testing.T) {
	for in, want := range map[string]bool{
		"LE FANTÔME DE L'OPÉRA": true,
		"Le Fantôme de l'Opéra": false,
		"1917":                  false,
	} {
		if got := isUpper(in); got != want {
			t.Errorf("isUpper(%q) = %v, want %v", in, got, want)
		}
	}
}
