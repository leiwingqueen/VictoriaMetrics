## server start
```shell
# start VictoriaMetrics with OpenTSDB support
./victoria-metrics-prod -opentsdbHTTPListenAddr=:4242

# index page
http://127.0.0.1:8428/

# send data
curl -H 'Content-Type: application/json' -d '{"metric":"x.y.z","value":45.34,"tags":{"t1":"v1","t2":"v2"}}' http://localhost:4242/api/put

curl -H 'Content-Type: application/json' -d '{"metric":"http_requests_total","value":10,"tags":{"job":"apiserver","handler":"/api/comments"}}' http://localhost:4242/api/put
```
