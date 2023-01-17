openssl req -new -newkey rsa:2048 -x509 -sha256 -days 365 -nodes -out localhost.crt -keyout localhost.key
mv localhost.* server