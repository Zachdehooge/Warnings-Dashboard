del custom-warnings.html

templ generate

go build -o weather-warnings.exe ./cmd

.\weather-warnings.exe --watch -i 30 -o custom-warnings.html -v