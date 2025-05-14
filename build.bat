del warnings.html /F

del weather-warnings.exe /F 

templ generate

go build -o weather-warnings.exe ./cmd

start "" ".\weather-warnings.exe" --watch -i 30 -o warnings.html -v 

timeout /t 5 

start warnings.html
