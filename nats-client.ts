import * as THREE from 'three';
import { GUI } from 'dat.gui';
// Use local package import
import { wsconnect, NatsConnection, StringCodec } from '@nats-io/nats-core';

// Global variables
let scene: THREE.Scene;
let camera: THREE.PerspectiveCamera;
let renderer: THREE.WebGLRenderer;
let gui: GUI;
let natsConnection: NatsConnection | null = null;
let messageObjects: THREE.Mesh[] = [];
let messageList: HTMLElement;
let connectionStatus: HTMLElement;

// Configuration
const config = {
    serverUrl: 'ws://localhost:19223',
    subject: 'demo.messages',
    messageText: 'Hello NATS!',
    sendMessage: () => sendMessage(),
    connect: () => connectToNats(),
    disconnect: () => disconnectFromNats(),
    clearMessages: () => clearMessages()
};

// Initialize the application
async function init() {
    try {
        console.log('Starting NATS client initialization...');
        
        // Get DOM elements
        messageList = document.getElementById('messageList')!;
        connectionStatus = document.getElementById('connectionStatus')!;
        
        console.log('DOM elements found:', { messageList: !!messageList, connectionStatus: !!connectionStatus });
        
        // Initialize Three.js
        initThreeJS();
        console.log('Three.js initialized');
        
        // Initialize GUI
        initGUI();
        console.log('GUI initialized');
        
        // Start render loop
        animate();
        console.log('Render loop started');
        
        console.log('NATS client initialized successfully');
    } catch (error) {
        console.error('Error initializing NATS client:', error);
    }
}

function initThreeJS() {
    // Create scene
    scene = new THREE.Scene();
    scene.background = new THREE.Color(0x0a0a0a);
    
    // Expose scene globally for testing
    (window as any).scene = scene;
    
    // Create camera
    camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 1000);
    camera.position.set(0, 0, 10);
    
    // Create renderer
    renderer = new THREE.WebGLRenderer({ antialias: true });
    renderer.setSize(window.innerWidth, window.innerHeight);
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    
    document.getElementById('container')!.appendChild(renderer.domElement);
    
    // Add lighting
    const ambientLight = new THREE.AmbientLight(0x404040, 0.6);
    scene.add(ambientLight);
    
    const directionalLight = new THREE.DirectionalLight(0xffffff, 0.8);
    directionalLight.position.set(10, 10, 5);
    directionalLight.castShadow = true;
    scene.add(directionalLight);
    
    // Add some initial geometry
    const geometry = new THREE.BoxGeometry(1, 1, 1);
    const material = new THREE.MeshLambertMaterial({ color: 0x444444 });
    const cube = new THREE.Mesh(geometry, material);
    cube.position.set(0, 0, 0);
    scene.add(cube);
    
    // Handle window resize
    window.addEventListener('resize', onWindowResize);
}

function initGUI() {
    gui = new GUI();
    
    const connectionFolder = gui.addFolder('Connection');
    connectionFolder.add(config, 'serverUrl');
    connectionFolder.add(config, 'connect');
    connectionFolder.add(config, 'disconnect');
    connectionFolder.open();
    
    const messageFolder = gui.addFolder('Messages');
    messageFolder.add(config, 'subject');
    messageFolder.add(config, 'messageText');
    messageFolder.add(config, 'sendMessage');
    messageFolder.add(config, 'clearMessages');
    messageFolder.open();
}

async function connectToNats() {
    try {
        if (natsConnection) {
            await disconnectFromNats();
        }
        
        updateStatus('Connecting...', 'orange');
        
        // Connect to NATS server via WebSocket
        natsConnection = await wsconnect({
            servers: [config.serverUrl]
        });
        
        updateStatus('Connected', 'green');
        
        // Subscribe to messages
        const subscription = natsConnection.subscribe(config.subject);
        
        // Process incoming messages
        (async () => {
            for await (const msg of subscription) {
                const message = new TextDecoder().decode(msg.data);
                console.log(`Received: ${message}`);
                addMessageToUI(message, 'received');
                createMessageVisualization(message, 'received');
            }
        })();
        
        console.log('Connected to NATS server');
        
    } catch (error) {
        console.error('Failed to connect to NATS:', error);
        console.error('Error details:', {
            message: error.message,
            stack: error.stack,
            name: error.name
        });
        updateStatus('Connection Failed', 'red');
    }
}

async function disconnectFromNats() {
    if (natsConnection) {
        await natsConnection.close();
        natsConnection = null;
        updateStatus('Disconnected', 'red');
        console.log('Disconnected from NATS server');
    }
}

async function sendMessage() {
    if (!natsConnection) {
        console.log('Not connected to NATS server');
        return;
    }
    
    try {
        const data = new TextEncoder().encode(config.messageText);
        
        natsConnection.publish(config.subject, data);
        
        console.log(`Sent: ${config.messageText}`);
        addMessageToUI(config.messageText, 'sent');
        createMessageVisualization(config.messageText, 'sent');
        
    } catch (error) {
        console.error('Failed to send message:', error);
        console.error('Send error details:', {
            message: error.message,
            stack: error.stack,
            name: error.name
        });
    }
}

function addMessageToUI(message: string, type: 'sent' | 'received') {
    const messageDiv = document.createElement('div');
    messageDiv.className = `message ${type}`;
    messageDiv.textContent = `${type.toUpperCase()}: ${message}`;
    messageList.appendChild(messageDiv);
    
    // Auto-scroll to bottom
    messageList.scrollTop = messageList.scrollHeight;
    
    // Limit number of messages shown
    const messages = messageList.children;
    if (messages.length > 50) {
        messageList.removeChild(messages[0]);
    }
}

function createMessageVisualization(message: string, type: 'sent' | 'received') {
    // Create a visual representation of the message
    const geometry = new THREE.SphereGeometry(0.2, 8, 6);
    const color = type === 'sent' ? 0x00ff00 : 0x0088ff;
    const material = new THREE.MeshLambertMaterial({ color });
    
    const messageMesh = new THREE.Mesh(geometry, material);
    
    // Position randomly around the center
    const angle = Math.random() * Math.PI * 2;
    const radius = 3 + Math.random() * 2;
    messageMesh.position.set(
        Math.cos(angle) * radius,
        Math.sin(angle) * radius,
        (Math.random() - 0.5) * 4
    );
    
    // Add some animation properties
    (messageMesh as any).velocity = new THREE.Vector3(
        (Math.random() - 0.5) * 0.02,
        (Math.random() - 0.5) * 0.02,
        (Math.random() - 0.5) * 0.02
    );
    
    (messageMesh as any).life = 300; // frames to live
    
    scene.add(messageMesh);
    messageObjects.push(messageMesh);
}

function clearMessages() {
    // Clear UI messages
    messageList.innerHTML = '';
    
    // Clear visual messages
    messageObjects.forEach(obj => {
        scene.remove(obj);
    });
    messageObjects = [];
}

function updateStatus(text: string, color: string) {
    connectionStatus.textContent = text;
    connectionStatus.style.color = color;
}

function animate() {
    requestAnimationFrame(animate);
    
    // Animate message objects
    messageObjects.forEach((obj, index) => {
        const mesh = obj as any;
        
        // Move the object
        obj.position.add(mesh.velocity);
        
        // Rotate
        obj.rotation.x += 0.01;
        obj.rotation.y += 0.01;
        
        // Fade out over time
        mesh.life--;
        if (mesh.life <= 0) {
            scene.remove(obj);
            messageObjects.splice(index, 1);
        } else {
            const alpha = mesh.life / 300;
            (obj.material as THREE.MeshLambertMaterial).opacity = alpha;
            (obj.material as THREE.MeshLambertMaterial).transparent = true;
        }
    });
    
    // Rotate the main cube
    const cube = scene.children.find(child => child instanceof THREE.Mesh && child.geometry instanceof THREE.BoxGeometry);
    if (cube) {
        cube.rotation.x += 0.005;
        cube.rotation.y += 0.01;
    }
    
    renderer.render(scene, camera);
}

function onWindowResize() {
    camera.aspect = window.innerWidth / window.innerHeight;
    camera.updateProjectionMatrix();
    renderer.setSize(window.innerWidth, window.innerHeight);
}

// Expose functions globally for testing
(window as any).connectToNats = connectToNats;
(window as any).disconnectFromNats = disconnectFromNats;
(window as any).sendMessage = sendMessage;
(window as any).config = config;

// Initialize when DOM is loaded
document.addEventListener('DOMContentLoaded', init);
