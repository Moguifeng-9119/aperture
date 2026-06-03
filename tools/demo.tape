# Aperture Demo
# Record with: vhs tools/demo.tape

Output tools/demo.gif

Set Theme "Dracula"
Set Width 1000
Set Height 600
Set FontSize 14
Set Padding 40
Set Framerate 24
Set PlaybackSpeed 1.0

Hide
Type "git clone https://github.com/Moguifeng-9119/aperture.git && cd aperture"
Enter
Sleep 500ms
Type "make build"
Enter
Sleep 2s
Type "cp config.example.yaml config.yaml"
Enter
Sleep 200ms
Type "export OPENAI_API_KEY=sk-..."
Enter
Sleep 200ms
Type "clear"
Enter

Show
Sleep 500ms
Type "./aperture"
Enter
Sleep 1s

# Split screen: show curl requests
Type "# Auto-routing: let Aperture pick the best model"
Enter
Sleep 500ms

Type 'curl -s http://localhost:8080/v1/chat/completions \'
Enter
Type '  -H "Content-Type: application/json" \'
Enter
Type '  -d '"'"'{"model":"auto","messages":[{"role":"user","content":"Hello!"}]}'"'"' | jq .'
Enter
Sleep 1s

Sleep 500ms
Type "# Show which model was chosen"
Enter
Sleep 200ms
Type 'curl -sI localhost:8080/v1/chat/completions \'
Enter
Type '  -d '"'"'{"model":"auto","messages":[{"role":"user","content":"Hello!"}]}'"'"' | grep X-Aperture'
Enter
Sleep 500ms

Type "# X-Aperture-Model: llama-3.1-8b-instant"
Enter
Type "# X-Aperture-Provider: groq"
Enter
Type "# X-Aperture-Reason: rule:greeting matched"
Enter
Sleep 1s

Type "# Dashboard: http://localhost:8080/dashboard"
Enter
Sleep 2s
