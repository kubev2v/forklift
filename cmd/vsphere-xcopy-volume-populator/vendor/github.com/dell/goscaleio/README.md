# Goscaleio
The *Goscaleio* project represents API bindings that can be used to provide ScaleIO functionality into other Go applications.


- [Current State](#state)
- [Usage](#usage)
- [Licensing](#licensing)
- [Support](#support)

## Use Cases
Any application written in Go can take advantage of these bindings.  Specifically, things that are involved in monitoring, management, and more specifically infrastructrue as code would find these bindings relevant.


## <a id="state">Current State</a>
Early build-out and pre-documentation stages.  The basics around authentication and object models are there.


## <a id="usage">Usage</a>

### Logging in

    client, err := goscaleio.NewClient()
    if err != nil {
      log.Fatalf("err: %v", err)
    }

    _, err = client.Authenticate(&goscaleio.ConfigConnect{endpoint, username, password})
    if err != nil {
      log.Fatalf("error authenticating: %v", err)
    }

    fmt.Println("Successfuly logged in to ScaleIO Gateway at", client.SIOEndpoint.String())


### Reusing the authentication token
Once a client struct is created via the ```NewClient()``` function, you can replace the ```Token``` with the saved token.

    client, err := goscaleio.NewClient()
    if err != nil {
      log.Fatalf("error with NewClient: %s", err)
    }

    client.SetToken(oldToken)

### Get Systems
Retrieving systems is the first step after authentication which enables you to work with other necessary methods.

#### All Systems

    systems, err := client.GetInstance()
    if err != nil {
      log.Fatalf("err: problem getting instance %v", err)
    }

#### Find a System

    system, err := client.FindSystem(systemid,"","")
    if err != nil {
      log.Fatalf("err: problem getting instance %v", err)
    }


### Get Protection Domains
Once you have a ```System``` struct you can then get other things like ```Protection Domains```.

    protectiondomains, err := system.GetProtectionDomain()
    if err != nil {
      log.Fatalf("error getting protection domains: %v", err)
    }

## Debugging

Two environment variables can be set to aid in debugging

Env Var | Default Value |
-- | -- |
`GOSCALEIO_DEBUG` | `false`
`GOSCALEIO_SHOWHTTP` | `false`

Setting `GOSCALEIO_DEBUG` well enable logging to `stdout`.
Setting `GOSCALEIO_SHOWHTTP` will log all HTTP requests and responses to `stdout`.


<a id="licensing">Licensing</a>
---------
Licensed under the Apache License, Version 2.0 (the “License”); you may not use this file except in compliance with the License. You may obtain a copy of the License at <http://www.apache.org/licenses/LICENSE-2.0>

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an “AS IS” BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

<a id="support">Support</a>
-------

For any issues, questions or feedback, please follow our [support process](https://github.com/dell/csm/blob/main/docs/SUPPORT.md)
