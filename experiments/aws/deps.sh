echo 'start' > 'setup.log'

go get golang.org/x/crypto/...
go get github.com/rs/zerolog

echo 'done' >> 'setup.log'
