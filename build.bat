del warnings.html

del weather-warnings.exe

templ generate

go build -o weather-warnings.exe ./cmd

.\weather-warnings.exe --watch -i 30 -o warnings.html -v