# http-dump-request

http-dump-request server and docker container for test or monitoring

## API

### dump request

```
% curl http://localhost:3000/ 
GET / HTTP/1.1
Host: localhost:3000
Accept: */*
User-Agent: curl/7.64.1
```

### whoami / hostname

```
% curl --compressed localhost:3000/whoami 
my-great-hostname
```

`/whoami.txt` is also supported.


### Status Code

```
% curl --compressed localhost:3000/demo/status/418
418 I'm a teapot
```

### fizzbuzz stream

fizzbuzz with chunked transfer and interval

```
%  curl -v localhost:3000/demo/fizzbuzz_stream
*   Trying ::1...
* TCP_NODELAY set
* Connected to localhost (::1) port 3000 (#0)
> GET /demo/fizzbuzz_stream HTTP/1.1
> Host: localhost:3000
> User-Agent: curl/7.64.1
> Accept: */*
> 
< HTTP/1.1 200 OK
< Vary: Accept-Encoding
< Date: Fri, 05 Mar 2021 07:47:06 GMT
< Content-Type: text/plain; charset=utf-8
< Transfer-Encoding: chunked
< 
#001
#002
#003 Fizz
#004
#005 Buzz
#006 Fizz
#007
#008
#009 Fizz
#010 Buzz
#011
#012 Fizz
#013
#014
#015 FizzBuzz
* Connection #0 to host localhost left intact
* Closing connection 0
```

`/demo/fizzbuzz` is also supported.

### basic auth

Use URI path as id and password

`/demo/basic/{id}/{password}`

```
%  curl --fail localhost:3000/demo/basic/fizz/buzz 
curl: (22) The requested URL returned error: 401 Unauthorized
```

```
%  curl --fail --user fizz:buzz localhost:3000/demo/basic/fizz/buzz
GET /demo/basic/fizz/buzz HTTP/1.1
Host: localhost:3000
Accept: */*
Authorization: Basic Zml6ejpidXp6
User-Agent: curl/7.64.1
```

### health check

```
% curl http://localhost:3000/live
OK
```

### version

```
% curl http://localhost:3000/version
0.1.x
```

## Run with docker

```
$ docker run -p 3000:3000 nomadscafe.sakuracr.jp/http-dump-request
```