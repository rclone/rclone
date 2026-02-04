#!/usr/bin/env node
// storacha-bridge.mjs - Node.js bridge for rclone Storacha backend
// This script runs as a subprocess and communicates via stdin/stdout JSON

import * as Client from '@storacha/client';
import * as readline from 'readline';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';

let client = null;
let currentSpace = null;

// Local metadata store (maps filename → {cid, size, modTime})
// Stored in ~/.config/rclone/storacha-meta/<spaceDID>.json
let metadataPath = null;
let metadata = {};

function loadMetadata(spaceDID) {
  const configDir = path.join(os.homedir(), '.config', 'rclone', 'storacha-meta');
  fs.mkdirSync(configDir, { recursive: true });
  metadataPath = path.join(configDir, `${spaceDID.replace(/:/g, '_')}.json`);
  
  try {
    if (fs.existsSync(metadataPath)) {
      metadata = JSON.parse(fs.readFileSync(metadataPath, 'utf8'));
    }
  } catch (e) {
    console.error(`[storacha] Failed to load metadata: ${e.message}`);
    metadata = {};
  }
}

function saveMetadata() {
  if (metadataPath) {
    try {
      fs.writeFileSync(metadataPath, JSON.stringify(metadata, null, 2));
    } catch (e) {
      console.error(`[storacha] Failed to save metadata: ${e.message}`);
    }
  }
}

// Read JSON requests from stdin, write JSON responses to stdout
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  terminal: false
});

// Handler functions
const handlers = {
  async init({ spaceDID, email }) {
    try {
      // Create client
      client = await Client.create();
      
      // Login if email provided and not already logged in
      if (email) {
        const accounts = client.accounts();
        if (Object.keys(accounts).length === 0) {
          console.error(`[storacha] Logging in with email: ${email}`);
          await client.login(email);
          console.error('[storacha] Check your email to authorize this agent');
        }
      }
      
      // Set space if provided
      if (spaceDID) {
        const spaces = client.spaces();
        currentSpace = spaces.find(s => s.did() === spaceDID);
        
        if (currentSpace) {
          await client.setCurrentSpace(spaceDID);
        } else {
          // Space not found, might need to claim delegations
          console.error(`[storacha] Space ${spaceDID} not found locally, claiming delegations...`);
          await client.capability.access.claim();
          
          // Try again
          const updatedSpaces = client.spaces();
          currentSpace = updatedSpaces.find(s => s.did() === spaceDID);
          
          if (currentSpace) {
            await client.setCurrentSpace(spaceDID);
          } else {
            throw new Error(`Space ${spaceDID} not found. Available spaces: ${updatedSpaces.map(s => s.did()).join(', ')}`);
          }
        }
        
        // Load metadata for this space
        loadMetadata(spaceDID);
      }
      
      return { initialized: true, spaceDID: currentSpace?.did() };
    } catch (error) {
      throw new Error(`Init failed: ${error.message}`);
    }
  },
  
  async upload({ name, data, size }) {
    if (!client) throw new Error('Client not initialized');
    if (!currentSpace) throw new Error('No space selected');
    
    // data comes as base64 from Go's JSON encoding of []byte
    const buffer = Buffer.from(data, 'base64');
    const file = new File([buffer], name, { type: 'application/octet-stream' });
    
    const cid = await client.uploadFile(file);
    const cidStr = cid.toString();
    
    // Store metadata locally
    metadata[name] = {
      cid: cidStr,
      size: buffer.length,
      modTime: new Date().toISOString()
    };
    saveMetadata();
    
    return { 
      cid: cidStr,
      size: buffer.length 
    };
  },
  
  async uploadDirectory({ files }) {
    if (!client) throw new Error('Client not initialized');
    if (!currentSpace) throw new Error('No space selected');
    
    const fileObjects = files.map(f => {
      const buffer = Buffer.from(f.data, 'base64');
      return new File([buffer], f.name, { type: 'application/octet-stream' });
    });
    
    const rootCid = await client.uploadDirectory(fileObjects);
    
    return { cid: rootCid.toString() };
  },
  
  async list({ path: dirPath }) {
    if (!client) throw new Error('Client not initialized');
    
    // Return files from local metadata that match the path
    const entries = [];
    const prefix = dirPath ? dirPath + '/' : '';
    
    for (const [name, info] of Object.entries(metadata)) {
      // Check if file is in the requested directory
      let relativeName = name;
      if (prefix && name.startsWith(prefix)) {
        relativeName = name.slice(prefix.length);
      } else if (prefix && !name.startsWith(prefix)) {
        continue; // Not in this directory
      }
      
      // If relativeName contains /, it's in a subdirectory
      const slashIndex = relativeName.indexOf('/');
      if (slashIndex !== -1) {
        // It's a subdirectory
        const subdir = relativeName.slice(0, slashIndex);
        if (!entries.find(e => e.name === subdir && e.isDir)) {
          entries.push({
            name: subdir,
            cid: '',
            size: 0,
            isDir: true,
            modTime: new Date().toISOString()
          });
        }
      } else {
        // It's a file in this directory
        entries.push({
          name: relativeName,
          cid: info.cid,
          size: info.size,
          isDir: false,
          modTime: info.modTime
        });
      }
    }
    
    return entries;
  },
  
  async stat({ name }) {
    // Get info about a specific file
    if (!metadata[name]) {
      return { found: false };
    }
    
    const info = metadata[name];
    return {
      found: true,
      name: name,
      cid: info.cid,
      size: info.size,
      modTime: info.modTime
    };
  },
  
  async download({ cid }) {
    if (!client) throw new Error('Client not initialized');
    
    // Fetch from IPFS gateway
    const gatewayUrl = `https://w3s.link/ipfs/${cid}`;
    const response = await fetch(gatewayUrl);
    
    if (!response.ok) {
      throw new Error(`Failed to fetch from gateway: ${response.statusText}`);
    }
    
    const buffer = await response.arrayBuffer();
    const data = Buffer.from(buffer).toString('base64');
    
    return { data };
  },
  
  async remove({ cid }) {
    if (!client) throw new Error('Client not initialized');
    
    // Parse CID and remove
    // Note: This removes the upload record, not the actual data from IPFS
    await client.remove(cid);
    
    return { removed: true };
  },
  
  async whoami() {
    if (!client) throw new Error('Client not initialized');
    
    const accounts = client.accounts();
    const spaces = client.spaces();
    
    return {
      accounts: Object.keys(accounts),
      spaces: spaces.map(s => ({ did: s.did(), name: s.name() })),
      currentSpace: currentSpace?.did()
    };
  }
};

// Process incoming requests
rl.on('line', async (line) => {
  let request;
  try {
    request = JSON.parse(line);
  } catch (e) {
    console.log(JSON.stringify({ 
      id: 0, 
      success: false, 
      error: `Invalid JSON: ${e.message}` 
    }));
    return;
  }
  
  const { id, method, params } = request;
  
  try {
    const handler = handlers[method];
    if (!handler) {
      throw new Error(`Unknown method: ${method}`);
    }
    
    const result = await handler(params || {});
    console.log(JSON.stringify({ id, success: true, result }));
  } catch (error) {
    console.log(JSON.stringify({ 
      id, 
      success: false, 
      error: error.message 
    }));
  }
});

// Handle process termination
process.on('SIGTERM', () => process.exit(0));
process.on('SIGINT', () => process.exit(0));

// Signal ready
console.error('[storacha-bridge] Ready');
