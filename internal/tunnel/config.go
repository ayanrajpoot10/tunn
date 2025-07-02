package tunnel

// Config holds all configuration parameters
type Config struct {
	Mode            string
	FrontDomain     string
	LocalSOCKSPort  int
	ProxyHost       string
	ProxyPort       string
	TargetHost      string
	TargetPort      string
	SSHUsername     string
	SSHPassword     string
	SSHPort         string
	PayloadTemplate string
}
