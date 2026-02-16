# Clean up old files
rm -f warnings.html
rm -f weather-warnings

# Build Go app
go build -o weather-warnings ./cmd

# Start the Go program in the background
./weather-warnings --watch -i 30 -o warnings.html