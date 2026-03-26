import * as vscode from 'vscode';
import * as net from 'net';

let client: net.Socket | null = null;
let isConnected = false;
let disposableListeners: vscode.Disposable[] = [];

export function activate(context: vscode.ExtensionContext) {
    console.log('Syntactic Sonar extension is now active!');

    const startCommand = vscode.commands.registerCommand('syntactic-sonar.start', () => {
        connectToServer();
    });

    const stopCommand = vscode.commands.registerCommand('syntactic-sonar.stop', () => {
        disconnectFromServer();
    });

    context.subscriptions.push(startCommand, stopCommand);
    
    connectToServer();
}

function connectToServer() {
    if (client && isConnected) {
        vscode.window.showInformationMessage('Syntactic Sonar is already connected.');
        return;
    }

    client = new net.Socket();
    client.connect(4445, '127.0.0.1', () => {
        console.log('Connected to Syntactic Sonar daemon');
        isConnected = true;
        vscode.window.showInformationMessage('Syntactic Sonar: Connected to daemon');
        registerEditorListeners();
    });

    client.on('error', (err) => {
        console.error('Syntactic Sonar Connection Error:', err);
        vscode.window.showErrorMessage('Syntactic Sonar Connection Error: ' + err.message);
        isConnected = false;
        unregisterEditorListeners();
    });

    client.on('close', () => {
        console.log('Syntactic Sonar Connection Closed');
        isConnected = false;
        unregisterEditorListeners();
    });
}

function disconnectFromServer() {
    if (client) {
        client.destroy();
        client = null;
    }
    isConnected = false;
    unregisterEditorListeners();
    vscode.window.showInformationMessage('Syntactic Sonar: Disconnected from daemon');
}

function registerEditorListeners() {
    unregisterEditorListeners();

    const changeSelection = vscode.window.onDidChangeTextEditorSelection((event) => {
        const editor = event.textEditor;
        const document = editor.document;
        
        if (event.selections.length > 0) {
            const position = event.selections[0].active;
            const content = document.getText();
            
            sendUpdate(document.fileName, position.line, position.character, content, document.languageId);
        }
    });

    disposableListeners.push(changeSelection);
}

function unregisterEditorListeners() {
    disposableListeners.forEach(d => d.dispose());
    disposableListeners = [];
}

function sendUpdate(file: string, line: number, col: number, content: string, languageId: string) {
    if (!client || !isConnected) return;

    const payload = {
        jsonrpc: "2.0",
        method: "update_sonar",
        params: {
            file: file,
            language: languageId,
            line: line,
            col: col,
            content: content
        }
    };

    const message = JSON.stringify(payload) + '\n';
    try {
        client.write(message);
    } catch (e) {
        console.error('Failed to write to Syntactic Sonar daemon:', e);
    }
}

export function deactivate() {
    disconnectFromServer();
}
