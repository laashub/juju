package ec2

import (
	"io/ioutil"
	"launchpad.net/goamz/aws"
	. "launchpad.net/gocheck"
	"launchpad.net/goyaml"
	"launchpad.net/juju-core/environs"
	"launchpad.net/juju-core/environs/config"
	"os"
	"path/filepath"
	"strings"
)

// Use local suite since this file lives in the ec2 package
// for testing internals.
type ConfigSuite struct {
	savedHome, savedAccessKey, savedSecretKey string
}

var _ = Suite(&ConfigSuite{})

var configTestRegion = aws.Region{
	Name:        "configtest",
	EC2Endpoint: "testregion.nowhere:1234",
}

var testAuth = aws.Auth{"gopher", "long teeth"}

// configTest specifies a config parsing test, checking that env when
// parsed as the ec2 section of a config file matches baseConfigResult
// when mutated by the mutate function, or that the parse matches the
// given error.
type configTest struct {
	config    attrs
	change    attrs
	region    string
	cbucket   string
	pbucket   string
	accessKey string
	secretKey string
	err       string
}

type attrs map[string]interface{}

func (t configTest) check(c *C) {
	envs := attrs{
		"environments": attrs{
			"testenv": attrs{
				"type": "ec2",
			},
		},
	}
	testenv := envs["environments"].(attrs)["testenv"].(attrs)
	for k, v := range t.config {
		testenv[k] = v
	}
	if _, ok := testenv["control-bucket"]; !ok {
		testenv["control-bucket"] = "x"
	}
	data, err := goyaml.Marshal(envs)
	c.Assert(err, IsNil)

	es, err := environs.ReadEnvironsBytes(data)
	c.Check(err, IsNil)

	e, err := es.Open("testenv")
	if t.change != nil {
		c.Assert(err, IsNil)

		// Testing a change in configuration.
		var old, changed, valid *config.Config
		ec2env := e.(*environ)
		old = ec2env.ecfg().Config
		changed, err = old.Apply(t.change)
		c.Assert(err, IsNil)

		// Keep err for validation below.
		valid, err = providerInstance.Validate(changed, old)
		if err == nil {
			err = ec2env.SetConfig(valid)
		}
	}
	if t.err != "" {
		c.Check(err, ErrorMatches, t.err)
		return
	}
	c.Assert(err, IsNil)

	ecfg := e.(*environ).ecfg()
	c.Assert(ecfg.Name(), Equals, "testenv")
	c.Assert(ecfg.controlBucket(), Equals, "x")
	if t.region != "" {
		c.Assert(ecfg.region(), Equals, t.region)
	}
	if t.pbucket != "" {
		c.Assert(ecfg.publicBucket(), Equals, t.pbucket)
	}
	if t.accessKey != "" {
		c.Assert(ecfg.accessKey(), Equals, t.accessKey)
		c.Assert(ecfg.secretKey(), Equals, t.secretKey)
	} else {
		c.Assert(ecfg.accessKey(), DeepEquals, testAuth.AccessKey)
		c.Assert(ecfg.secretKey(), DeepEquals, testAuth.SecretKey)
	}
}

var configTests = []configTest{
	{
		config: attrs{},
	}, {
		config: attrs{
			"region": "eu-west-1",
		},
		region: "eu-west-1",
	}, {
		config: attrs{
			"region": "unknown",
		},
		err: ".*invalid region name.*",
	}, {
		config: attrs{
			"region": "configtest",
		},
		region: "configtest",
	}, {
		config: attrs{
			"region": "configtest",
		},
		change: attrs{
			"region": "us-east-1",
		},
		err: `cannot change region from "configtest" to "us-east-1"`,
	}, {
		config: attrs{
			"region": 666,
		},
		err: ".*expected string, got 666",
	}, {
		config: attrs{
			"access-key": 666,
		},
		err: ".*expected string, got 666",
	}, {
		config: attrs{
			"secret-key": 666,
		},
		err: ".*expected string, got 666",
	}, {
		config: attrs{
			"control-bucket": 666,
		},
		err: ".*expected string, got 666",
	}, {
		change: attrs{
			"control-bucket": "new-x",
		},
		err: `cannot change control-bucket from "x" to "new-x"`,
	}, {
		config: attrs{
			"public-bucket": 666,
		},
		err: ".*expected string, got 666",
	}, {
		config: attrs{
			"public-bucket": "foo",
		},
		pbucket: "foo",
	}, {
		config: attrs{
			"access-key": "jujuer",
			"secret-key": "open sesame",
		},
		accessKey: "jujuer",
		secretKey: "open sesame",
	}, {
		config: attrs{
			"access-key": "jujuer",
		},
		err: ".*environment has no access-key or secret-key",
	}, {
		config: attrs{
			"secret-key": "badness",
		},
		err: ".*environment has no access-key or secret-key",
	},
}

func indent(s string, with string) string {
	var r string
	lines := strings.Split(s, "\n")
	for _, l := range lines {
		r += with + l + "\n"
	}
	return r
}

func (s *ConfigSuite) SetUpTest(c *C) {
	s.savedHome = os.Getenv("HOME")
	s.savedAccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	s.savedSecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")

	home := c.MkDir()
	sshDir := filepath.Join(home, ".ssh")
	err := os.Mkdir(sshDir, 0777)
	c.Assert(err, IsNil)
	err = ioutil.WriteFile(filepath.Join(sshDir, "id_rsa.pub"), []byte("sshkey\n"), 0666)
	c.Assert(err, IsNil)

	os.Setenv("HOME", home)
	os.Setenv("AWS_ACCESS_KEY_ID", testAuth.AccessKey)
	os.Setenv("AWS_SECRET_ACCESS_KEY", testAuth.SecretKey)
	aws.Regions["configtest"] = configTestRegion
}

func (s *ConfigSuite) TearDownTest(c *C) {
	os.Setenv("HOME", s.savedHome)
	os.Setenv("AWS_ACCESS_KEY_ID", s.savedAccessKey)
	os.Setenv("AWS_SECRET_ACCESS_KEY", s.savedSecretKey)
	delete(aws.Regions, "configtest")
}

func (s *ConfigSuite) TestConfig(c *C) {
	for i, t := range configTests {
		c.Logf("test %d: %q", i, t.config)
		t.check(c)
	}
}

func (s *ConfigSuite) TestMissingAuth(c *C) {
	os.Setenv("AWS_ACCESS_KEY_ID", "")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "")
	test := configTests[0]
	test.err = "environment has no access-key or secret-key"
	test.check(c)
}
