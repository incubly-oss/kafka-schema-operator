package schemareg_mock

import "testing"

type args struct {
	schema string
}

func Test_parseAvroSchema(t *testing.T) {
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "simple JSON string",
			args:    args{`"string"`},
			want:    `"string"`,
			wantErr: false,
		},
		{
			name:    "complex JSON string",
			args:    args{`"some other \"string\"[]{}"`},
			want:    `"some other \"string\"[]{}"`,
			wantErr: false,
		},
		{
			name:    "invalid string",
			args:    args{`string`},
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty array",
			args:    args{`[]`},
			want:    `[]`,
			wantErr: false,
		},
		{
			name:    "invalid array",
			args:    args{`[`},
			want:    "",
			wantErr: true,
		},
		{
			name:    "array of strings",
			args:    args{`["foo","bar"]`},
			want:    `["foo","bar"]`,
			wantErr: false,
		},
		{
			name:    "array of numbers",
			args:    args{`[1,1.23]`},
			want:    `[1,1.23]`,
			wantErr: false,
		},
		{
			name:    "array of bools",
			args:    args{`[true, false]`},
			want:    `[true, false]`,
			wantErr: false,
		},
		{
			name:    "array of objects",
			args:    args{`[{"foo":1}, {"bar":2}]`},
			want:    `[{"foo":1}, {"bar":2}]`,
			wantErr: false,
		},
		{
			name:    "array of arrays",
			args:    args{`[[1,2], ["a","b"]]`},
			want:    `[[1,2], ["a","b"]]`,
			wantErr: false,
		},
		{
			name:    "array of different types",
			args:    args{`[[], {}, true, 1, "foo"]`},
			want:    `[[], {}, true, 1, "foo"]`,
			wantErr: false,
		},
		{
			name:    "empty object",
			args:    args{`{}`},
			want:    `{}`,
			wantErr: false,
		},
		{
			name:    "invalid object",
			args:    args{`{xxx}`},
			want:    "",
			wantErr: true,
		},
		{
			name:    "object with different fields",
			args:    args{`{"foo":true,"bar":"baz", "qux":[1,2,3.45]}`},
			want:    `{"foo":true,"bar":"baz", "qux":[1,2,3.45]}`,
			wantErr: false,
		},
		{
			name:    "null",
			args:    args{"null"},
			want:    "null",
			wantErr: false,
		},
		{
			name:    "true",
			args:    args{"true"},
			want:    "true",
			wantErr: false,
		},
		{
			name:    "false",
			args:    args{"false"},
			want:    "false",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAvroSchema(tt.args.schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAvroSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseAvroSchema() got = %v, want %v", got, tt.want)
			}
		})
	}
}
