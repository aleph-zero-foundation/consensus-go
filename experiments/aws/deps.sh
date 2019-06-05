echo 'start' > 'setup.log'

sudo chmod 666 .cache/go-build/log.txt
go get golang.org/x/crypto/...
go get github.com/rs/zerolog

echo 'done' >> 'setup.log'
