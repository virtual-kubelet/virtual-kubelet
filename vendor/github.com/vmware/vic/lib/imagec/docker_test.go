// Copyright 2017 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package imagec

import (
	"testing"

	"github.com/docker/docker/reference"
	"github.com/stretchr/testify/assert"
)

const (
	UbuntuTaggedRef      = "library/ubuntu:latest"
	UbuntuDigest         = "ubuntu@sha256:45b23dee08af5e43a7fea6c4cf9c25ccf269ee113168c19722f87876677c5cb2"
	UbuntuDigestSHA      = "sha256:45b23dee08af5e43a7fea6c4cf9c25ccf269ee113168c19722f87876677c5cb2"
	UbuntuDigestManifest = `{
   "schemaVersion": 1,
   "name": "library/ubuntu",
   "tag": "14.04",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:28a2f68d1120598986362662445c47dce7ec13c2662479e7aab9f0ecad4a7416"
      },
      {
         "blobSum": "sha256:fd2731e4c50ce221d785d4ce26a8430bca9a95bfe4162fafc997a1cc65682cce"
      },
      {
         "blobSum": "sha256:5a132a7e7af11f304041e93efb9cb2a0a7839bccaec5a03cfbdc9a3f5d0eb481"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"id\":\"56063ad57855f2632f641a622efa81a0feda1731bfadda497b1288d11feef4da\",\"parent\":\"4e1f7c524148bf80fcc4ce9991e88d708048d38440e3e3a14d56e72c17ddccc7\",\"created\":\"2016-03-03T21:38:53.80360049Z\",\"container\":\"b6361ab0a2a82f71c5bd3becbb9c854331f8259b9c3fe466bf6e7e073c735a2c\",\"container_config\":{\"Hostname\":\"c24ffee5b2b8\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [\\\"/bin/bash\\\"]\"],\"Image\":\"4e1f7c524148bf80fcc4ce9991e88d708048d38440e3e3a14d56e72c17ddccc7\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"docker_version\":\"1.9.1\",\"config\":{\"Hostname\":\"c24ffee5b2b8\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[],\"Cmd\":[\"/bin/bash\"],\"Image\":\"4e1f7c524148bf80fcc4ce9991e88d708048d38440e3e3a14d56e72c17ddccc7\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\"}"
      },
      {
         "v1Compatibility": "{\"id\":\"4e1f7c524148bf80fcc4ce9991e88d708048d38440e3e3a14d56e72c17ddccc7\",\"parent\":\"38112156678df7d8001ae944f118d283009565540dc0cd88fb39fccc88c3c7f2\",\"created\":\"2016-03-03T21:38:53.085760873Z\",\"container\":\"ccc6ec8b31df981344b4107bd890394be35564adb8d13df34d1cb1849c7f0c3e\",\"container_config\":{\"Hostname\":\"c24ffee5b2b8\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[],\"Cmd\":[\"/bin/sh\",\"-c\",\"sed -i 's/^#\\\\s*\\\\(deb.*universe\\\\)$/\\\\1/g' /etc/apt/sources.list\"],\"Image\":\"38112156678df7d8001ae944f118d283009565540dc0cd88fb39fccc88c3c7f2\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"docker_version\":\"1.9.1\",\"config\":{\"Hostname\":\"c24ffee5b2b8\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[],\"Cmd\":null,\"Image\":\"38112156678df7d8001ae944f118d283009565540dc0cd88fb39fccc88c3c7f2\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":1895}"
      },
      {
         "v1Compatibility": "{\"id\":\"38112156678df7d8001ae944f118d283009565540dc0cd88fb39fccc88c3c7f2\",\"parent\":\"454970bd163ba95435b50e963edd63b2b2fff4c1845e5d3cd03d5ba8afb8a08d\",\"created\":\"2016-03-03T21:38:51.45368726Z\",\"container\":\"3c8556d1a209f22cfbc87f3cbd9bcb6674c5f9645a14aa488756d129f6987f40\",\"container_config\":{\"Hostname\":\"c24ffee5b2b8\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[],\"Cmd\":[\"/bin/sh\",\"-c\",\"echo '#!/bin/sh' \\u003e /usr/sbin/policy-rc.d \\t\\u0026\\u0026 echo 'exit 101' \\u003e\\u003e /usr/sbin/policy-rc.d \\t\\u0026\\u0026 chmod +x /usr/sbin/policy-rc.d \\t\\t\\u0026\\u0026 dpkg-divert --local --rename --add /sbin/initctl \\t\\u0026\\u0026 cp -a /usr/sbin/policy-rc.d /sbin/initctl \\t\\u0026\\u0026 sed -i 's/^exit.*/exit 0/' /sbin/initctl \\t\\t\\u0026\\u0026 echo 'force-unsafe-io' \\u003e /etc/dpkg/dpkg.cfg.d/docker-apt-speedup \\t\\t\\u0026\\u0026 echo 'DPkg::Post-Invoke { \\\"rm -f /var/cache/apt/archives/*.deb /var/cache/apt/archives/partial/*.deb /var/cache/apt/*.bin || true\\\"; };' \\u003e /etc/apt/apt.conf.d/docker-clean \\t\\u0026\\u0026 echo 'APT::Update::Post-Invoke { \\\"rm -f /var/cache/apt/archives/*.deb /var/cache/apt/archives/partial/*.deb /var/cache/apt/*.bin || true\\\"; };' \\u003e\\u003e /etc/apt/apt.conf.d/docker-clean \\t\\u0026\\u0026 echo 'Dir::Cache::pkgcache \\\"\\\"; Dir::Cache::srcpkgcache \\\"\\\";' \\u003e\\u003e /etc/apt/apt.conf.d/docker-clean \\t\\t\\u0026\\u0026 echo 'Acquire::Languages \\\"none\\\";' \\u003e /etc/apt/apt.conf.d/docker-no-languages \\t\\t\\u0026\\u0026 echo 'Acquire::GzipIndexes \\\"true\\\"; Acquire::CompressionTypes::Order:: \\\"gz\\\";' \\u003e /etc/apt/apt.conf.d/docker-gzip-indexes\"],\"Image\":\"454970bd163ba95435b50e963edd63b2b2fff4c1845e5d3cd03d5ba8afb8a08d\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"docker_version\":\"1.9.1\",\"config\":{\"Hostname\":\"c24ffee5b2b8\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[],\"Cmd\":null,\"Image\":\"454970bd163ba95435b50e963edd63b2b2fff4c1845e5d3cd03d5ba8afb8a08d\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":194533}"
      },
      {
         "v1Compatibility": "{\"id\":\"454970bd163ba95435b50e963edd63b2b2fff4c1845e5d3cd03d5ba8afb8a08d\",\"created\":\"2016-03-03T21:38:46.169812943Z\",\"container\":\"c24ffee5b2b808674d335e2c42c9c47aa9aff1b368eb5920018cde7dd26f2046\",\"container_config\":{\"Hostname\":\"c24ffee5b2b8\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ADD file:b9504126dc55908988977286e89c43c7ea73a506d43fae82c29ef132e21b6ece in /\"],\"Image\":\"\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.9.1\",\"config\":{\"Hostname\":\"c24ffee5b2b8\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":187763841}"
      }
   ],
   "signatures": [
      {
         "header": {
            "jwk": {
               "crv": "P-256",
               "kid": "G2JA:NPRD:EWRM:EFEJ:4PHQ:TRZR:6W6O:D5LC:UJ36:RHOE:ZN7D:N55I",
               "kty": "EC",
               "x": "NrETepARqTLeOBcTdBCE8K8jbQJgiTH1p7XJ78zBxjk",
               "y": "ay0SmJatkJs-JdnW80807CcNPWHElsh6MW_JTh7NdbU"
            },
            "alg": "ES256"
         },
         "signature": "kHbWdD1NQw2RIAQ8uYnKmolU3Z_WeUW8DfRtJRprVDzK7AZaF-ChI4V9Lh74HnjSNwoNZ_QRhUQDl_Nezb0Hgw",
         "protected": "eyJmb3JtYXRMZW5ndGgiOjY3NzksImZvcm1hdFRhaWwiOiJDbjAiLCJ0aW1lIjoiMjAxNy0wNS0xN1QxODowMjozMFoifQ"
      }
   ]
}
`
)

func TestGetManifestDigest(t *testing.T) {
	// Get the manifest content when the image is not pulled by digest
	ref, err := reference.ParseNamed(UbuntuTaggedRef)
	if err != nil {
		t.Errorf(err.Error())
	}
	digest, err := getManifestDigest([]byte(UbuntuDigestManifest), ref)
	assert.NoError(t, err)
	assert.Equal(t, digest, UbuntuDigestSHA)

	// Get and verify the manifest content with the correct digest
	ref, err = reference.ParseNamed(UbuntuDigest)
	if err != nil {
		t.Errorf(err.Error())
	}
	_, ok := ref.(reference.Canonical)
	assert.True(t, ok)
	digest, err = getManifestDigest([]byte(UbuntuDigestManifest), ref)
	assert.NoError(t, err)
	assert.Equal(t, digest, UbuntuDigestSHA)

	// Attempt to get and verify an incorrect manifest content with the digest
	digest, err = getManifestDigest([]byte(DefaultManifest), ref)
	assert.NotNil(t, err)
}
