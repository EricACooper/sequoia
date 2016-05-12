# sequoia
Scalable couchbase testing with docker
<p align="center">
  <img src="arch.png" width="350"/>
</p>

**Install Docker**
* Mac - https://docs.docker.com/mac/
* Ubuntu - https://docs.docker.com/engine/installation/linux/ubuntulinux/
* CentOS - https://docs.docker.com/engine/installation/linux/centos/

**Install Go**
* Ubuntu - https://github.com/golang/go/wiki/Ubuntu
* CentOS - http://itekblog.com/centos-golang/
* Any - https://golang.org/dl/ 
 
**Build**
```bash
go get github.com/couchbaselabs/sequoia
cd $GOPATH/src/github.com/couchbaselabs/sequoia/
go build
```

## Testing

In Sequoia a test consists of a scope spec and a test spec.  The top-level config.yml file denotes which files to use for the test.  Alternetaively, command line args can be used to explicitely specify which scope and test to use when testing. 

```bash
# use defaults from config.yml
./sequoia -config config.xml

# run with command line args
./sequoia -scope tests/simple/scope_medium.yml -test tests/simple/test_small.yml
```

Refer to [Test Syntax](https://github.com/couchbaselabs/sequoia/wiki/Test-Syntax) for more information about how to build out your test and scopes.

## Client

Sequoia works by running containers that apply load to couchbase servers.  These containers are running on docker specified by the client in your config file.  Depending on your docker install you will need to use http(s) and specify port.  It's recommended to run over a tcp port.  

```yaml
# config.yml
...
client:  https://192.168.99.100:2376
```

Or on server without https and daemon running on port 2375

```yams
# config.yml
...
client:  http://172.23.97.124:2375
```


## Providers

Providers help decouple test and provisioning from the mechanisms that provide couchbase resources so that the same scope can present an identical environment to different tests.  You can change your provider via the config file.

```yaml
# config.yml
...
provider: docker  # dev, file
```

Read More about [Providers Here](https://github.com/couchbaselabs/sequoia/wiki/Providers)





