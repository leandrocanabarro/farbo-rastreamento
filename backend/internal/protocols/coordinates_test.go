package protocols

import (
	"math"
	"testing"
)

func TestDDMMToDecimal(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		hemisphere string
		want       float64
		wantErr    bool
	}{
		{name: "exemplo da especificação", value: "5257.4318", hemisphere: "N", want: 52 + 57.4318/60},
		{name: "latitude sul", value: "2332.5000", hemisphere: "S", want: -(23 + 32.5/60)},
		{name: "longitude com três dígitos de grau", value: "04633.1234", hemisphere: "W", want: -(46 + 33.1234/60)},
		{name: "longitude leste", value: "01122.3344", hemisphere: "E", want: 11 + 22.3344/60},
		{name: "hemisfério ausente é positivo", value: "1000.0000", hemisphere: "", want: 10},
		{name: "minutos inválidos", value: "1099.0000", hemisphere: "N", wantErr: true},
		{name: "curta demais", value: "12", hemisphere: "N", wantErr: true},
		{name: "vazia", value: "", hemisphere: "N", wantErr: true},
		{name: "hemisfério inválido", value: "1000.0000", hemisphere: "X", wantErr: true},
		{name: "não numérica", value: "abcd.1234", hemisphere: "N", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DDMMToDecimal(tc.value, tc.hemisphere)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("esperava erro, recebi %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("esperava %.9f, recebi %.9f", tc.want, got)
			}
		})
	}
}

func TestKnotsToKmh(t *testing.T) {
	if got := KnotsToKmh(10); math.Abs(got-18.52) > 1e-9 {
		t.Fatalf("esperava 18.52, recebi %v", got)
	}
}

func TestValidCoordinates(t *testing.T) {
	if !ValidCoordinates(-23.5, -46.6) {
		t.Fatal("São Paulo deveria ser válida")
	}
	if ValidCoordinates(91, 0) || ValidCoordinates(0, 181) {
		t.Fatal("coordenada fora de faixa deveria ser recusada")
	}
	if ValidCoordinates(math.NaN(), 0) {
		t.Fatal("NaN deveria ser recusado")
	}
}

func TestValidateIMEI(t *testing.T) {
	if err := ValidateIMEI("869247061234567"); err != nil {
		t.Fatalf("IMEI válido recusado: %v", err)
	}
	if err := ValidateIMEI("12345"); err == nil {
		t.Fatal("IMEI curto deveria ser recusado")
	}
	if err := ValidateIMEI("86924706123456A"); err == nil {
		t.Fatal("IMEI com letra deveria ser recusado")
	}
}
