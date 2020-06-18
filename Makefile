filemarket: $(shell find . -name "*.go") bindata.go
	go build -ldflags="-s -w" -o ./filemarket

bindata.go: static/bundle.js static/index.html static/global.css static/bundle.css
	go-bindata -o bindata.go static/...

static/bundle.js: $(shell find client)
	./node_modules/.bin/rollup -c

deploy: filemarket
	ssh root@nusakan-58 'systemctl stop filemarket'
	scp filemarket nusakan-58:filemarket/filemarket
	ssh root@nusakan-58 'systemctl start filemarket'
