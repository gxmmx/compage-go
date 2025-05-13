package certutils

import (
	"encoding/pem"
	"os"
	"testing"
)

func TestLoadPem(t *testing.T) {

	// -------------------------------------
	// Variables
	// -------------------------------------

	// Sources
	validCertMultiline := `-----BEGIN CERTIFICATE-----
MIIDiDCCAnCgAwIBAgIIGD7BdtKfAkgwDQYJKoZIhvcNAQELBQAwczELMAkGA1UE
BhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lz
Y28xEDAOBgNVBAoTB1Rlc3RPcmcxCzAJBgNVBAsTAklUMRgwFgYDVQQDEw9zb21l
LmRvbWFpbi5jb20wHhcNMjUwNTEyMTA0MDIxWhcNMjUwNTEzMTA0MDIxWjBzMQsw
CQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy
YW5jaXNjbzEQMA4GA1UEChMHVGVzdE9yZzELMAkGA1UECxMCSVQxGDAWBgNVBAMT
D3NvbWUuZG9tYWluLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
ALV99ktLAuykXD5HdU6r0HaybO9EI4tjbH7BCGuuoskiuR1uO6hCqHjMRdk+ed3C
+be3LG7BctOeaFMqjzBnEH3z1CGg2izO21/9+IA1And3Udo/AUn/1Q355ufQnoX9
Zo9fJXaG3lipLtAb6BCn1DBHiZl0JyzSzU9xg3z50RMcmwfMILykGQLBOOLKfZS5
MMcUiYyAOfk0rTncjTOqjt4+vBIP7hw2KitPsl69SzOib0uN7IEuUhHq+dkdRb3V
Z3yIxdSBFInquHCsAvmyaAZY1tGXJEakGdpc9Kqm5+ASZ1gcq4cv8mZxL54PXVSC
D9bFLIzGiQOt9DpDQMJ3tqMCAwEAAaMgMB4wDgYDVR0PAQH/BAQDAgWgMAwGA1Ud
EwEB/wQCMAAwDQYJKoZIhvcNAQELBQADggEBAHuJ7D6Y4/A7AqMK3hFujCsTLFvY
djid/RfZ3I/XZTyBCMjO8FxLxUZgaAubjXfG54V95cDRMnkmpH3XFlARLvC+fgZs
7NUPa952SjIV5gd78v8exrUfabqvnFDqK19C5Vo5C3dDG9WM0UAC8b3GJefg+IvS
sX/UsxFtmoaDSzBKSHTCaRr/PcrxnggeiVK0IgT92rEHLGZNoZsiFdZ5PV6Uhscj
yEjRNWFqEb8SqseH1D/DHAknnUxXjvWXdhpMU85Nm2/qEYzClA5q3lamJs0zo66v
LY2Ehrz+0UwKniemz6zpavaQI57KsBSOlu5HyIN9klMXKjLRYfqvQ/99Y7k=
-----END CERTIFICATE-----`
	validCert := "-----BEGIN CERTIFICATE-----\nMIIDiDCCAnCgAwIBAgIIGD7BdtKfAkgwDQYJKoZIhvcNAQELBQAwczELMAkGA1UE\nBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lz\nY28xEDAOBgNVBAoTB1Rlc3RPcmcxCzAJBgNVBAsTAklUMRgwFgYDVQQDEw9zb21l\nLmRvbWFpbi5jb20wHhcNMjUwNTEyMTA0MDIxWhcNMjUwNTEzMTA0MDIxWjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEQMA4GA1UEChMHVGVzdE9yZzELMAkGA1UECxMCSVQxGDAWBgNVBAMT\nD3NvbWUuZG9tYWluLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB\nALV99ktLAuykXD5HdU6r0HaybO9EI4tjbH7BCGuuoskiuR1uO6hCqHjMRdk+ed3C\n+be3LG7BctOeaFMqjzBnEH3z1CGg2izO21/9+IA1And3Udo/AUn/1Q355ufQnoX9\nZo9fJXaG3lipLtAb6BCn1DBHiZl0JyzSzU9xg3z50RMcmwfMILykGQLBOOLKfZS5\nMMcUiYyAOfk0rTncjTOqjt4+vBIP7hw2KitPsl69SzOib0uN7IEuUhHq+dkdRb3V\nZ3yIxdSBFInquHCsAvmyaAZY1tGXJEakGdpc9Kqm5+ASZ1gcq4cv8mZxL54PXVSC\nD9bFLIzGiQOt9DpDQMJ3tqMCAwEAAaMgMB4wDgYDVR0PAQH/BAQDAgWgMAwGA1Ud\nEwEB/wQCMAAwDQYJKoZIhvcNAQELBQADggEBAHuJ7D6Y4/A7AqMK3hFujCsTLFvY\ndjid/RfZ3I/XZTyBCMjO8FxLxUZgaAubjXfG54V95cDRMnkmpH3XFlARLvC+fgZs\n7NUPa952SjIV5gd78v8exrUfabqvnFDqK19C5Vo5C3dDG9WM0UAC8b3GJefg+IvS\nsX/UsxFtmoaDSzBKSHTCaRr/PcrxnggeiVK0IgT92rEHLGZNoZsiFdZ5PV6Uhscj\nyEjRNWFqEb8SqseH1D/DHAknnUxXjvWXdhpMU85Nm2/qEYzClA5q3lamJs0zo66v\nLY2Ehrz+0UwKniemz6zpavaQI57KsBSOlu5HyIN9klMXKjLRYfqvQ/99Y7k=\n-----END CERTIFICATE-----"
	validKey := "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCznO0BTSiFGr37\nTfvWriNZ/2fXyBv7WAsZTQP+eltEsfIeCxY1MJJv6PZ0mfWchh6X3mBpfFSWSV/I\nn8IFwAQ7/8cVzrAUKa67dFQMKb0GkZ19K8+uAQU2/Fm2QmobQSliEt4JnBAE4o7L\nxObZO1F2tdSh6W1QZnXjdDR6RBomZ8yYZxE1Ltsc2m58hj7z4egNTGh4qtQwXoBy\nV2C8uYxtYn6TLf5quK3AElEWiIj1q4PaQdogkncWg8wOurrBi4w5qRZ1SNwzYnOf\nZlwpMZWX/wq/g90AGKhDdTJBJdDATIjDiO4ghuyl5JoRGrN84rvsEZYXTjRGMAsr\neiI8bxHBAgMBAAECggEAGlTSYws0ekT4HpgpaCjZymcYymfOAFxBLkmd4Qf/itPa\nhqCB9jTDdxAMV349pV99Ky8A6938CZdCqUcQeubSSBiAj3ggVKhisx0D+E+DJGwj\nDjdmyB/iq5O1tDRK2WmFX1kcP4TnTUwIeqOIY0bgs7pG7KCrs6p9tGV0DwueWMxG\nLzoLwL5fueQwVa4icDaXa5nUE1NlUnBHp18SyEIahoJBFCRIHoYaPGYO0O9gXnO8\n3VJRg77CKUQrIz4vaQNyCLQrqv/Vy7OL2tjEUEJvc2xg9dkLedaEJXcAwDx0gn7d\nFwLO2a/iMiQ2IY2R8i2+h1XqBfsYSOMnpPnOTbbkFQKBgQDqvE3DIeHyW9VqCioV\nXSb0bxAn0v2ZHjsGeUKrSOVeYrUxKr9DUKd7iU4ZnWS3AzQxGpN5DukuQlHtTkql\nlNSS1zlCRu43RFTo9BypeQtV/IVAtvh3rZ+Z8eK6qNZE+Awn7NGDdX7JKLiFOfvZ\nHIAto+7vy+Sg8loGNF5rbyTwPQKBgQDD4kmnNYQeZ1IpktWpRQ/kkKxKa/2dgKIh\nuhKGxk4nYJ7AdTS5XJuH55v08MBmIBs+Eg5z4Pd1C729YYoGPBl5LxzhFFd3M/z1\n0ZtmrpxVjaouDaIzu2NtWV5X7yRNEUhWE9K9Aie8emWbkjeqlqSM4XrQxQ7GadOY\nBeeAidvb1QKBgHzsZf4ZRCQ1V2itrCPehWLE0LZBBZG9kvApDKAXlWob4g4ej9eF\nTvzh39yl9PmpDNetKxrcIqDpzqwaZIOmp1LWk7SABzsGdKHdeHuA3dWPJGOCfM1E\na5IENwPb4tylneKJmB78ItNvhnPwneW300d23SxlOHGnSN3QdQd8CQ6JAoGBAK2b\njPOpNqNLp0I5ZSxUjTViE5ESDQDe1NNXerwAXZwAwjKIrmXqcd4No+d+yMa6heqJ\nTk3dgPQ3p76FCDNmaJ1C6DGEOdDoPrYPQ8/Jybz5hW6znqKC3ig4IKmGxYGYY6gG\ngawKkPU29X7gJH4IbWZ/IL6PJ/0qJeKCuR7vD/DRAoGARpLE39+obHC/089b23n7\n6fRDrHxNBBRdyD3W6zUkrKdX6+2tvszX4FPybaTsFUb+qq5xRQZc86+o/eOTsEWU\ncWE9UJmewOpQrWo+PYCECB5dT+2v7lt1iFCHI/R8pyvQciGpoVkx68rF/O4ki8i9\nVsyyTyx6dHMD0cdwmAaNRBk=\n-----END PRIVATE KEY-----"
	validRequest := "-----BEGIN CERTIFICATE REQUEST-----\nMIICuDCCAaACAQAwczELMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWEx\nFjAUBgNVBAcTDVNhbiBGcmFuY2lzY28xEDAOBgNVBAoTB1Rlc3RPcmcxCzAJBgNV\nBAsTAklUMRgwFgYDVQQDEw9zb21lLmRvbWFpbi5jb20wggEiMA0GCSqGSIb3DQEB\nAQUAA4IBDwAwggEKAoIBAQCrnjo7bRSTIhkq6NyOLpbheAy/uLu2TeVKAGYBMsWj\nKIoOiWrsppvvn1HdvrrUCybWm2inRIxYuY2rK1OZ7mcrr53O20Eg26/PqpW37xJ8\nBNgjGorNDq559wx66bjPlW4BoEf7VgjVWMFXvPuB3JtwbVyxJgc9LNWkX6X6a40a\n5RW3I42FFjBY3kkRhTMmYuXpAW7ey+CPD5fYbK+aMBXKK+hLrZAMJToDPBmO50C1\n/SOItTZ9HoCBvUf8z3niCaxD0LHsffycIbYYKkO3n2nKTeDlEJFI24wI+61VsrL5\nj/AygKsi3I9+oe03ruw/lW1KbPfBrmSMasUEvh3gWkmVAgMBAAGgADANBgkqhkiG\n9w0BAQsFAAOCAQEABczK+n/uGlvzkwi/wyECf5R1rsnbrNagQxgsUwKH2VIC/9CG\nep/Jjxn5HFugtLS+ijUW2O/v3dD9vBYqaY+Xtx7ji8VH5aUKcGwN93jBj5RXcAy4\nQ8aRSU1IqDeCs81VQdBmAvPYvhQVRrfwcRqqig+vYu8JD+RGzVUG8y+2xelNiCNj\nrcik6jB3FJl5q5wAjUt7sY8Fmu1Yq/vtTHr5RUbyr0GghfXwHEQCU+124zTfOnfP\nRnk7eNJOQ/FsoKe1dMqisLy8VnN6n7AmLdBf0FqLvaB4doyRiYnoePUwIT9Ecy4b\nQh2TOfsQ4xaHEBxASAFKEqW0LhlmvPQCVZAKOw==\n-----END CERTIFICATE REQUEST-----"
	validKeyPKCS1 := "-----BEGIN RSA PRIVATE KEY-----\nMIIEogIBAAKCAQEAqPuyt7tLCukq6EIdOc3EDwN7oalPUTJfEykqN6yeyZO+Yb5y\n5lSMPCC7hPtEHuKjLhIgNrub9Wgf3pJS/diWqwnOc2V3h5XlaK7FxKtqxcpKJQtk\nsLb3NJMH9+LpkfiilKEHH7fuf3577+iMpHLnEYWBTF0uL1YLVO/UVPRG+CbW1/Q7\nf7vG2FNOg8ccczOmJ6Pzygzbn4Jxx0t79//YYOT7NpPVMmSTjwNQ9CUSjBOqQyxg\nfT9+Wqic82GndnqXzDJgwjUt+GAxPuu4LAKGq+2Nll+JfXrAj9XrRUDgXFTXfw+U\nSHmBtgGHC0A836jm12yc3h2m/xcl73+zMWqX4wIDAQABAoIBAAInDqbrglsLDv+y\nvpI6w57DVxhUl/eQp4hkHlm437RGpwayDomjlb5lq63X0OLUf+qkKCKPr5QSivwN\nVh74sCv77dQnfJHOwC4zXRPnxm5APkM24BtQ1q5QmdSNC0JPmKvsWd22vJmFgB5V\nZ9vnwMzG1sJPxOOtJMDPZfTdFb2vJP6ogIymHmS84Ijaj0MxCUHFHQlaBQS+V0fW\nJZ31G4mvgY2fdzrQj+c9A9b/izmEihJ9e3TqZ9wcJUYdntx3p0a+Cw6AuvLLnOKb\nRleCL6ejjiCZ++4fwCbd4bHKcsESCREmbHHK0PpoanUvw4VYy+US2aM9jjUGbgW6\nYhtfTUECgYEAwgUrhurof5SA3W3cvW72BjT8GeQ04hsa+oDc9CHFefqz9n4+g6Zs\nV1ZhnQ17+47ByohEEwb2PP/udAIjAP68eY5koRTYEnh6eTcYwqutKnFXWHqs8W0T\nUNNLEzRpUs+t9VlhygNHTkHb/SHIoW0KxyQ67HgT00he2BI9LKA3YKMCgYEA3vcF\nQX2sRRx89OyxjfV7LLUpUONSeCIdjmx6c/mTtP0MgVU6ol1fM79TiN0wqX9PCRFD\nZLJHLXf1lUMm8XeizYXRsdAUjIrq+o6ZEceEr/JBcJnZYBFR1d2MQZogHMynC7E0\n5AN+UfKRuXZN1udNWaUqn1oSLHaw0+xdnMWdH8ECgYBaWbv+VTA2ETq9YubTlHOf\nSldH21zBGmxC0XWTfpKOji/2Dq4f8oUrWr+UOm5NJBqcrT4+OhS7LVem0EPqt7Wf\nSa8U0Dcayt4FyqGOLhIy3JsXSfF1cBz5m5uvcs3FUY8p3RjL0SEIkWTXiT775WKK\ngBWsfvKEhoQcTMoOGQIQzwKBgG+kAlazhYGpbQv6REFPjFhrcX+WA2Ixutjoijvt\n2L7EAfH0agKIfDXd9AbQsh/8pedlZHhUJ/2lVithz2sSu0rrWX8OMGva1yOUKSLU\n4yRyScAG2OgYZACRCTyD3tZsxqu9FD2jXinMKplRmlIjyQA9CmV15SmsWIgUjx8D\nd9RBAoGAem493B9BxveZHH9e1gPPA6yrk1x3ffVT1O79GjZO/sAjneuAR4TSHoUB\nKtM8vs7QgbASRthFK977t7AObC+0hb99K82Hn4gxMxOkT5pHqmze2lZPpACJ81d0\nK+rIBsafk45lsDEoTJGhHrP2lc8na1u120GrKnQdGFzW/TmkLOU=\n-----END RSA PRIVATE KEY-----"
	validCertB64 := "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tXG5NSUlEaURDQ0FuQ2dBd0lCQWdJSUdEN0JkdEtmQWtnd0RRWUpLb1pJaHZjTkFRRUxCUUF3Y3pFTE1Ba0dBMVVFXG5CaE1DVlZNeEV6QVJCZ05WQkFnVENrTmhiR2xtYjNKdWFXRXhGakFVQmdOVkJBY1REVk5oYmlCR2NtRnVZMmx6XG5ZMjh4RURBT0JnTlZCQW9UQjFSbGMzUlBjbWN4Q3pBSkJnTlZCQXNUQWtsVU1SZ3dGZ1lEVlFRREV3OXpiMjFsXG5MbVJ2YldGcGJpNWpiMjB3SGhjTk1qVXdOVEV5TVRBME1ESXhXaGNOTWpVd05URXpNVEEwTURJeFdqQnpNUXN3XG5DUVlEVlFRR0V3SlZVekVUTUJFR0ExVUVDQk1LUTJGc2FXWnZjbTVwWVRFV01CUUdBMVVFQnhNTlUyRnVJRVp5XG5ZVzVqYVhOamJ6RVFNQTRHQTFVRUNoTUhWR1Z6ZEU5eVp6RUxNQWtHQTFVRUN4TUNTVlF4R0RBV0JnTlZCQU1UXG5EM052YldVdVpHOXRZV2x1TG1OdmJUQ0NBU0l3RFFZSktvWklodmNOQVFFQkJRQURnZ0VQQURDQ0FRb0NnZ0VCXG5BTFY5OWt0TEF1eWtYRDVIZFU2cjBIYXliTzlFSTR0amJIN0JDR3V1b3NraXVSMXVPNmhDcUhqTVJkaytlZDNDXG4rYmUzTEc3QmN0T2VhRk1xanpCbkVIM3oxQ0dnMml6TzIxLzkrSUExQW5kM1Vkby9BVW4vMVEzNTV1ZlFub1g5XG5abzlmSlhhRzNsaXBMdEFiNkJDbjFEQkhpWmwwSnl6U3pVOXhnM3o1MFJNY213Zk1JTHlrR1FMQk9PTEtmWlM1XG5NTWNVaVl5QU9mazByVG5jalRPcWp0NCt2QklQN2h3MktpdFBzbDY5U3pPaWIwdU43SUV1VWhIcStka2RSYjNWXG5aM3lJeGRTQkZJbnF1SENzQXZteWFBWlkxdEdYSkVha0dkcGM5S3FtNStBU1oxZ2NxNGN2OG1aeEw1NFBYVlNDXG5EOWJGTEl6R2lRT3Q5RHBEUU1KM3RxTUNBd0VBQWFNZ01CNHdEZ1lEVlIwUEFRSC9CQVFEQWdXZ01Bd0dBMVVkXG5Fd0VCL3dRQ01BQXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSHVKN0Q2WTQvQTdBcU1LM2hGdWpDc1RMRnZZXG5kamlkL1JmWjNJL1haVHlCQ01qTzhGeEx4VVpnYUF1YmpYZkc1NFY5NWNEUk1ua21wSDNYRmxBUkx2QytmZ1pzXG43TlVQYTk1MlNqSVY1Z2Q3OHY4ZXhyVWZhYnF2bkZEcUsxOUM1Vm81QzNkREc5V00wVUFDOGIzR0plZmcrSXZTXG5zWC9Vc3hGdG1vYURTekJLU0hUQ2FSci9QY3J4bmdnZWlWSzBJZ1Q5MnJFSExHWk5vWnNpRmRaNVBWNlVoc2NqXG55RWpSTldGcUViOFNxc2VIMUQvREhBa25uVXhYanZXWGRocE1VODVObTIvcUVZekNsQTVxM2xhbUpzMHpvNjZ2XG5MWTJFaHJ6KzBVd0tuaWVtejZ6cGF2YVFJNTdLc0JTT2x1NUh5SU45a2xNWEtqTFJZZnF2US85OVk3az1cbi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0="
	validCertB64multiline := "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURpRENDQW5DZ0F3SUJBZ0lJR0Q3QmR0S2ZBa2d3RFFZSktvWklodmNOQVFFTEJRQXdjekVMTUFrR0ExVUUKQmhNQ1ZWTXhFekFSQmdOVkJBZ1RDa05oYkdsbWIzSnVhV0V4RmpBVUJnTlZCQWNURFZOaGJpQkdjbUZ1WTJsegpZMjh4RURBT0JnTlZCQW9UQjFSbGMzUlBjbWN4Q3pBSkJnTlZCQXNUQWtsVU1SZ3dGZ1lEVlFRREV3OXpiMjFsCkxtUnZiV0ZwYmk1amIyMHdIaGNOTWpVd05URXlNVEEwTURJeFdoY05NalV3TlRFek1UQTBNREl4V2pCek1Rc3cKQ1FZRFZRUUdFd0pWVXpFVE1CRUdBMVVFQ0JNS1EyRnNhV1p2Y201cFlURVdNQlFHQTFVRUJ4TU5VMkZ1SUVaeQpZVzVqYVhOamJ6RVFNQTRHQTFVRUNoTUhWR1Z6ZEU5eVp6RUxNQWtHQTFVRUN4TUNTVlF4R0RBV0JnTlZCQU1UCkQzTnZiV1V1Wkc5dFlXbHVMbU52YlRDQ0FTSXdEUVlKS29aSWh2Y05BUUVCQlFBRGdnRVBBRENDQVFvQ2dnRUIKQUxWOTlrdExBdXlrWEQ1SGRVNnIwSGF5Yk85RUk0dGpiSDdCQ0d1dW9za2l1UjF1TzZoQ3FIak1SZGsrZWQzQworYmUzTEc3QmN0T2VhRk1xanpCbkVIM3oxQ0dnMml6TzIxLzkrSUExQW5kM1Vkby9BVW4vMVEzNTV1ZlFub1g5ClpvOWZKWGFHM2xpcEx0QWI2QkNuMURCSGlabDBKeXpTelU5eGczejUwUk1jbXdmTUlMeWtHUUxCT09MS2ZaUzUKTU1jVWlZeUFPZmswclRuY2pUT3FqdDQrdkJJUDdodzJLaXRQc2w2OVN6T2liMHVON0lFdVVoSHErZGtkUmIzVgpaM3lJeGRTQkZJbnF1SENzQXZteWFBWlkxdEdYSkVha0dkcGM5S3FtNStBU1oxZ2NxNGN2OG1aeEw1NFBYVlNDCkQ5YkZMSXpHaVFPdDlEcERRTUozdHFNQ0F3RUFBYU1nTUI0d0RnWURWUjBQQVFIL0JBUURBZ1dnTUF3R0ExVWQKRXdFQi93UUNNQUF3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUh1SjdENlk0L0E3QXFNSzNoRnVqQ3NUTEZ2WQpkamlkL1JmWjNJL1haVHlCQ01qTzhGeEx4VVpnYUF1YmpYZkc1NFY5NWNEUk1ua21wSDNYRmxBUkx2QytmZ1pzCjdOVVBhOTUyU2pJVjVnZDc4djhleHJVZmFicXZuRkRxSzE5QzVWbzVDM2RERzlXTTBVQUM4YjNHSmVmZytJdlMKc1gvVXN4RnRtb2FEU3pCS1NIVENhUnIvUGNyeG5nZ2VpVkswSWdUOTJyRUhMR1pOb1pzaUZkWjVQVjZVaHNjagp5RWpSTldGcUViOFNxc2VIMUQvREhBa25uVXhYanZXWGRocE1VODVObTIvcUVZekNsQTVxM2xhbUpzMHpvNjZ2CkxZMkVocnorMFV3S25pZW16NnpwYXZhUUk1N0tzQlNPbHU1SHlJTjlrbE1YS2pMUllmcXZRLzk5WTdrPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0t"
	validCertTrailingNewline := "-----BEGIN CERTIFICATE-----\nMIIDiDCCAnCgAwIBAgIIGD7BdtKfAkgwDQYJKoZIhvcNAQELBQAwczELMAkGA1UE\nBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lz\nY28xEDAOBgNVBAoTB1Rlc3RPcmcxCzAJBgNVBAsTAklUMRgwFgYDVQQDEw9zb21l\nLmRvbWFpbi5jb20wHhcNMjUwNTEyMTA0MDIxWhcNMjUwNTEzMTA0MDIxWjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEQMA4GA1UEChMHVGVzdE9yZzELMAkGA1UECxMCSVQxGDAWBgNVBAMT\nD3NvbWUuZG9tYWluLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB\nALV99ktLAuykXD5HdU6r0HaybO9EI4tjbH7BCGuuoskiuR1uO6hCqHjMRdk+ed3C\n+be3LG7BctOeaFMqjzBnEH3z1CGg2izO21/9+IA1And3Udo/AUn/1Q355ufQnoX9\nZo9fJXaG3lipLtAb6BCn1DBHiZl0JyzSzU9xg3z50RMcmwfMILykGQLBOOLKfZS5\nMMcUiYyAOfk0rTncjTOqjt4+vBIP7hw2KitPsl69SzOib0uN7IEuUhHq+dkdRb3V\nZ3yIxdSBFInquHCsAvmyaAZY1tGXJEakGdpc9Kqm5+ASZ1gcq4cv8mZxL54PXVSC\nD9bFLIzGiQOt9DpDQMJ3tqMCAwEAAaMgMB4wDgYDVR0PAQH/BAQDAgWgMAwGA1Ud\nEwEB/wQCMAAwDQYJKoZIhvcNAQELBQADggEBAHuJ7D6Y4/A7AqMK3hFujCsTLFvY\ndjid/RfZ3I/XZTyBCMjO8FxLxUZgaAubjXfG54V95cDRMnkmpH3XFlARLvC+fgZs\n7NUPa952SjIV5gd78v8exrUfabqvnFDqK19C5Vo5C3dDG9WM0UAC8b3GJefg+IvS\nsX/UsxFtmoaDSzBKSHTCaRr/PcrxnggeiVK0IgT92rEHLGZNoZsiFdZ5PV6Uhscj\nyEjRNWFqEb8SqseH1D/DHAknnUxXjvWXdhpMU85Nm2/qEYzClA5q3lamJs0zo66v\nLY2Ehrz+0UwKniemz6zpavaQI57KsBSOlu5HyIN9klMXKjLRYfqvQ/99Y7k=\n-----END CERTIFICATE-----\n"
	invalidCert := "-----BEGIN CERTIFICATE-----\nMIIDiDCCAnCgAwIBAgIIGD7BdtKfAkgwDQYJKoZIhvcNAQELBQAwczELMAkGA1UE\nBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lz\nY28xEDAOBgNVBAoTB1Rlc3RPcmcxCzAJBgNVBAsTAklUMRgwFgYDVQQDEw9zb21l\nLmRvbWFpbi5jb20wHhcNMjUwNTEyMTA0MDIxWhcNMjUwNTEzMTA0MDIxWjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEQMA4GA1UEChMHVGVzdE9yZzELMAkGA1UECxMCSVQxGDAWBgNVBAMT\nD3NvbWUuZG9tYWluLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB\nALV99ktLAuykXD5HdU6r0HaybO9EI4tjbH7BCGuuoskiuR1uO6hCqHjMRdk+ed3C\n+be3LG7BctOeaFMqjzBnEH3z1CGg2izO21/9+IA1And3Udo/AUn/1Q355ufQnoX9\nZo9fJXaG3lipLtAb6BCn1DBHiZl0JyzSzU9xg3z50RMcmwfMILykGQLBOOLKfZS5\nMMcUiYyAOfk0rTncjTOqjt4+vBIP7hw2KitPsl69SzOib0uN7IEuUhHq+dkdRb3V\nZ3yIxdSBFInquHCsAvmyaAZY1tGXJEakGdpc9Kqm5+ASZ1gcq4cv8mZxL54PXVSC\nD9bFLIzGiQOt9DpDQMJ3tqMCAwEAAaMgMB4wDgYDVR0PAQH/BAQDAgWgMAwGA1Ud\nEwEB/wQCMAAwDQYJKoZIhvcNAQELBQADggEBAHuJ7D6Y4/A7AqMK3hFujCsTLFvY\ndjid/RfZ3I/XZTyBCMjO8FxLxUZgaAubjXfG54V95cDRMnkmpH3XFlARLvC+fgZs\n7NUPa952SjIV5gd78v8exrUfabqvnFDqK19C5Vo5C3dDG9WM0UAC8b3GJefg+IvS\nsX/UsxFtmoaDSzBKSHTCaRr/PcrxnggeiVK0IgT92rEHLGZNoZsiFdZ5PV6Uhscj\nyEjRNWFqEb8SqseH1D/DHAknnUxXjvWXdhpMU85Nm2/qEYzClA5q3lamJs0zo66v\nLY2Ehrz+0UwKniemz6zpavaQI57KsBSOlu5HyIN9klMXKjLRYfqvQ/99Y7k\n-----END CERTIFICATE-----"
	invalidCertB64encoded := "c29tZSBpbnZhbGlkIHN0cmluZyB0aGF0IGlzIG5vdCBhIGNlcnRpZmljYXRl"
	invalidCertParseData := "-----BEGIN CERTIFICATE-----\nVGhpcyBpcyBub3QgYSB2YWxpZCBjZXJ0aWZpY2F0ZSBkZXJzZXJpYWw=\n-----END CERTIFICATE-----"
	invalidRequestParseData := "-----BEGIN CERTIFICATE REQUEST-----\nVGhpcyBpcyBub3QgYSB2YWxpZCBjZXJ0aWZpY2F0ZSBkZXJzZXJpYWw=\n-----END CERTIFICATE REQUEST-----"
	invalidKeyParseData := "-----BEGIN PRIVATE KEY-----\nVGhpcyBpcyBub3QgYSB2YWxpZCBjZXJ0aWZpY2F0ZSBkZXJzZXJpYWw=\n-----END PRIVATE KEY-----"

	// -------------------------------------
	// Temporary files
	// -------------------------------------

	// Create temp files for testing
	tmpDir := t.TempDir()
	// Crt file
	crtFileStd, err := os.CreateTemp(tmpDir, "certstd-*.crt")
	if err != nil {
		t.Fatalf("unexpected error: failed to create temp file: %v", err)
	}
	defer os.Remove(crtFileStd.Name())
	crtFileStd.WriteString(validCertMultiline)
	crtFileStd.Close()

	// Crt file with base64 content
	crtFileB64, err := os.CreateTemp(tmpDir, "certb64-*.crt")
	if err != nil {
		t.Fatalf("unexpected error: failed to create temp file: %v", err)
	}
	defer os.Remove(crtFileB64.Name())
	crtFileB64.WriteString(validCertB64)
	crtFileB64.Close()

	// Unreadable file
	unreadableFile, err := os.CreateTemp(tmpDir, "unreadable-*.crt")
	if err != nil {
		t.Fatalf("unexpected error: failed to create temp file: %v", err)
	}
	defer os.Remove(unreadableFile.Name())
	unreadableFile.WriteString(validCertB64)
	unreadableFile.Close()
	err = os.Chmod(unreadableFile.Name(), 0000)
	if err != nil {
		t.Fatalf("unexpected error: failed to change file permissions: %v", err)
	}

	// Empty file
	emptyFile, err := os.CreateTemp(tmpDir, "empty-*.crt")
	if err != nil {
		t.Fatalf("unexpected error: failed to create temp file: %v", err)
	}
	defer os.Remove(emptyFile.Name())
	emptyFile.Close()

	// -------------------------------------
	// Test matrix definition
	// -------------------------------------

	//Test matrix
	testMatrix := []struct {
		name  string
		input string
		src   int
		crts  int
		keys  int
		csr   int
		err   bool
	}{
		{
			name:  "Empty",
			input: "",
		},
		{
			name:  "Valid Certificate Multiline",
			input: validCertMultiline,
			src:   1,
			crts:  1,
		},
		{
			name:  "Valid Certificate Single line",
			input: validCert,
			src:   1,
			crts:  1,
		},
		{
			name:  "Valid Certificate B64",
			input: validCertB64,
			src:   2,
			crts:  1,
		},
		{
			name:  "Valid Certificate B64 Multiline",
			input: validCertB64multiline,
			src:   2,
			crts:  1,
		},
		{
			name:  "Valid Certificate Trailing Newline",
			input: validCertTrailingNewline,
			src:   1,
			crts:  1,
		},
		{
			name:  "Valid multiple certificates",
			input: validCertTrailingNewline + validCert,
			src:   1,
			crts:  2,
		},
		{
			name:  "Valid Certificate with Key",
			input: validCertTrailingNewline + validKey,
			src:   1,
			crts:  1,
			keys:  1,
		},
		{
			name:  "Valid Certificate Request",
			input: validRequest,
			src:   1,
			csr:   1,
		},
		{
			name:  "Valid Key",
			input: validKey,
			src:   1,
			keys:  1,
		},
		{
			name:  "Valid Key PKCS1",
			input: validKeyPKCS1,
			src:   1,
			keys:  1,
		},
		{
			name:  "Valid certificate from file",
			input: crtFileStd.Name(),
			src:   3,
			crts:  1,
		},
		{
			name:  "Valid certificate from file with base64 content",
			input: crtFileB64.Name(),
			src:   4,
			crts:  1,
		},
		{
			name:  "Invalid Certificate",
			input: invalidCert,
			src:   1,
			err:   true,
		},
		{
			name:  "Invalid Certificate B64",
			input: invalidCertB64encoded,
			src:   5,
			err:   true,
		},
		{
			name:  "Unreadable file",
			input: unreadableFile.Name(),
			src:   5,
			err:   true,
		},
		{
			name:  "Empty file",
			input: emptyFile.Name(),
			src:   5,
			err:   true,
		},
		{
			name:  "Unparsable Certificate",
			input: invalidCertParseData,
			src:   1,
			err:   true,
		},
		{
			name:  "Unparsable Certificate Request",
			input: invalidRequestParseData,
			src:   1,
			err:   true,
		},
		{
			name:  "Unparsable Key",
			input: invalidKeyParseData,
			src:   1,
			err:   true,
		},
	}

	// -----------------------------------------------------------------------------
	// Tests
	// -----------------------------------------------------------------------------

	// Initial content check
	t.Run("check common name", func(t *testing.T) {
		_, crts, _, _, err := LoadPem(validCert)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(crts) != 1 {
			t.Fatalf("expected 1 cert, got %d", len(crts))
		}
		if crts[0].Subject.CommonName != "some.domain.com" {
			t.Fatalf("expected common name 'some.domain.com', got '%s'", crts[0].Subject.CommonName)
		}
	})

	// Run test matrix
	for _, test := range testMatrix {
		t.Run(test.name, func(t *testing.T) {
			src, crts, keys, csr, err := LoadPem(test.input)
			if err != nil && !test.err {
				t.Errorf("unexpected error: %v", err)
			}
			if err == nil && test.err {
				t.Errorf("expected error but got none")
			}
			if src != test.src {
				t.Errorf("expected source %d, got %d", test.src, src)
			}
			if len(crts) != test.crts {
				t.Errorf("expected %d certs, got %d", test.crts, len(crts))
			}
			if len(keys) != test.keys {
				t.Errorf("expected %d keys, got %d", test.keys, len(keys))
			}
			if len(csr) != test.csr {
				t.Errorf("expected %d csr, got %d", test.csr, len(csr))
			}
		})
	}

	// Test edge cases
	t.Run("unsupported key", func(t *testing.T) {
		block := &pem.Block{
			Type:  "UNSUPPORTED KEY",
			Bytes: []byte("unsupported key"),
		}
		_, err := parseKeyBlock(block)
		if err == nil {
			t.Errorf("expected error but got none")
		}
	})
}
