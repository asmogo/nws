package protocol

import (
	"net/url"
	"reflect"
	"testing"
)

type args struct {
	s string
}

type parseTest struct {
	name    string
	args    args
	want    *URL
	wantErr bool
}

func TestParse(t *testing.T) {
	t.Parallel()
	for _, test := range createParseTests() {
		testCopy := test
		t.Run(testCopy.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(testCopy.args.s)
			if (err != nil) != testCopy.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, testCopy.wantErr)
				return
			}
			if !reflect.DeepEqual(got, testCopy.want) {
				t.Errorf("Parse() got = %v, want %v", got, testCopy.want)
			}
		})
	}
}

func createParseTests() []parseTest {
	return []parseTest{
		{name: "1", args: args{s: "http://D1Q78S3J78NIURJFEDQ74BJQCLH6AP35CKN66R3FELI0.9B7NTQSU4PBM2JJQJ0CMGHUENQON4GB28RLGQCH3D3NK2AQVFE70.nostr"}, want: &URL{IsDomain: true, TLD: "nostr", Name: "9b7ntqsu4pbm2jjqj0cmghuenqon4gb28rlgqch3d3nk2aqvfe70", SubName: "d1q78s3j78niurjfedq74bjqclh6ap35ckn66r3feli0", URL: &url.URL{Host: "d1q78s3j78niurjfedq74bjqclh6ap35ckn66r3feli0.9b7ntqsu4pbm2jjqj0cmghuenqon4gb28rlgqch3d3nk2aqvfe70.nostr", Scheme: "http"}}, wantErr: false},   //nolint:lll
		{name: "1", args: args{s: "http://d1q78s3j78niurjfedq74bjqclh6ap35ckn66r3feli0.9b7ntqsu4pbm2jjqj0cmghuenqon4gb28rlgqch3d3nk2aqvfe70.nostr"}, want: &URL{IsDomain: true, TLD: "nostr", Name: "9b7ntqsu4pbm2jjqj0cmghuenqon4gb28rlgqch3d3nk2aqvfe70", SubName: "d1q78s3j78niurjfedq74bjqclh6ap35ckn66r3feli0", URL: &url.URL{Host: "d1q78s3j78niurjfedq74bjqclh6ap35ckn66r3feli0.9b7ntqsu4pbm2jjqj0cmghuenqon4gb28rlgqch3d3nk2aqvfe70.nostr", Scheme: "http"}}, wantErr: false},   //nolint:lll
		{name: "1", args: args{s: "https://d1q78s3j78niurjfedq74bjqclh6ap35ckn66r3feli0.9b7ntqsu4pbm2jjqj0cmghuenqon4gb28rlgqch3d3nk2aqvfe70.nostr"}, want: &URL{IsDomain: true, TLD: "nostr", Name: "9b7ntqsu4pbm2jjqj0cmghuenqon4gb28rlgqch3d3nk2aqvfe70", SubName: "d1q78s3j78niurjfedq74bjqclh6ap35ckn66r3feli0", URL: &url.URL{Host: "d1q78s3j78niurjfedq74bjqclh6ap35ckn66r3feli0.9b7ntqsu4pbm2jjqj0cmghuenqon4gb28rlgqch3d3nk2aqvfe70.nostr", Scheme: "https"}}, wantErr: false}, //nolint:lll

	}
}
