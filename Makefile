server: web server/main.go src/App.js build
	cd server; go run main.go

web: src/App.js
	cd src; npm run build

clean:
	rm -rf build