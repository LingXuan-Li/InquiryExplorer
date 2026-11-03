package splash

const splashHTML = `
<!DOCTYPE html>
<html lang="ja">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Inquiry Explorer</title>

    <style>
        * {
            box-sizing: border-box;
        }

        body {
            margin: 0;
            background: #111827;
            color: #e5e7eb;
            font-family: sans-serif;
        }

        .container {
            padding: 24px;
        }

        header {
            display: flex;
            align-items: center;
            gap: 14px;
            margin-bottom: 24px;
        }

        .logo {
            width: 48px;
            height: 48px;
            display: grid;
            place-items: center;
            border-radius: 12px;
            background: #253b60;
            color: #8cb8ff;
            font-size: 20px;
            font-weight: bold;
        }

        h1 {
            font-size: 18px;
            margin: 0;
        }

        .version {
            font-size: 11px;
            color: #9ca3af;
            margin-top: 5px;
        }

        .log-title {
            font-size: 11px;
            color: #8cb8ff;
            font-weight: bold;
            margin-bottom: 10px;
        }

        .logs {
            height: 155px;
            overflow-y: auto;
            padding: 12px;
            background: #090f19;
            border-radius: 8px;
            font-family: monospace;
            font-size: 11px;
            line-height: 1.8;
            color: #a7f3d0;
            overflow-wrap: anywhere;
        }

        footer {
            display: flex;
            justify-content: space-between;
            margin-top: 16px;
            color: #6b7280;
            font-size: 10px;
        }
    </style>
</head>

<body>
    <main class="container">
        <header>
            <div class="logo">IE</div>

            <div>
                <h1>Inquiry Explorer</h1>
                <div class="version">Version 1.0.0</div>
            </div>
        </header>

        <div class="log-title">STARTUP LOG</div>

        <div id="logs" class="logs">
            <div>Initializing...</div>
        </div>

        <footer>
            <span>Inquiry Explorer</span>
            <span>Starting...</span>
        </footer>
    </main>

    <script>
		window.addLog = function(message) {
			const logs = document.getElementById("logs");

			const line = document.createElement("div");
			line.textContent = message;

			logs.appendChild(line);

			while (logs.children.length > 500) {
				logs.removeChild(logs.firstChild);
			}

			logs.scrollTop = logs.scrollHeight;
		};
    </script>
</body>
</html>
`
