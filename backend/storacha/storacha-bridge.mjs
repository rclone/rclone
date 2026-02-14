#!/usr/bin/env node
// storacha-bridge.mjs - Node.js bridge for rclone Storacha backend
// This script runs as a subprocess and communicates via stdin/stdout JSON

import * as Client from '@storacha/client';
import * as readline from 'readline';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import * as dagPB from '@ipld/dag-pb';
import { UnixFS } from 'ipfs-unixfs';
import { CID } from 'multiformats/cid';
import * as Block from 'multiformats/block';
import { sha256 } from 'multiformats/hashes/sha2';
import { CarWriter } from '@ipld/car';

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
    if (!currentSpace) throw new Error('No space selected');
    
    console.error(`[storacha] list called with path: "${dirPath}"`);
    
    const entries = [];
    let foundExactMatch = false;
    
    try {
      // Use client.capability.upload.list() with cursor pagination
      let cursor;
      do {
        const res = await client.capability.upload.list({ cursor });
        if (!res || !res.results) break;
        
        for (const upload of res.results) {
          const rootCID = upload.root.toString();
          const size = upload.shards?.reduce((sum, s) => sum + (s.size || 0), 0) || 0;
          
          // Use root CID as the name since we don't have filename mapping
          // If dirPath is specified, only return exact match or items in that directory
          if (dirPath) {
            // Exact match - return only this CID
            if (rootCID === dirPath || rootCID === dirPath.replace(/\/$/, '')) {
              entries.push({
                name: rootCID,
                cid: rootCID,
                size: size,
                isDir: false,
                modTime: upload.insertedAt || new Date().toISOString()
              });
              // Found exact match, stop all searching
              foundExactMatch = true;
              break;
            }
            // Prefix match for directory listing (only if path ends with /)
            if (dirPath.endsWith('/')) {
              const prefix = dirPath;
              if (rootCID.startsWith(prefix)) {
                entries.push({
                  name: rootCID,
                  cid: rootCID,
                  size: size,
                  isDir: false,
                  modTime: upload.insertedAt || new Date().toISOString()
                });
              }
            }
          } else {
            // No path specified - list all
            entries.push({
              name: rootCID,
              cid: rootCID,
              size: size,
              isDir: false,
              modTime: upload.insertedAt || new Date().toISOString()
            });
          }
        }
        
        // Stop pagination if we found exact match
        if (foundExactMatch) break;
        
        cursor = res.cursor;
      } while (cursor);
      
      return entries;
    } catch (error) {
      console.error(`[storacha] List failed: ${error.message}`);
      return [];
    }
  },
  
  async stat({ name }) {
    if (!client) throw new Error('Client not initialized');
    
    // Use client.capability.upload.list() to find the file by CID
    try {
      let cursor;
      do {
        const res = await client.capability.upload.list({ cursor });
        if (!res || !res.results) break;
        
        for (const upload of res.results) {
          const rootCID = upload.root.toString();
          if (rootCID === name) {
            const size = upload.shards?.reduce((sum, s) => sum + (s.size || 0), 0) || 0;
            return {
              found: true,
              name: rootCID,
              cid: rootCID,
              size: size,
              modTime: upload.insertedAt || new Date().toISOString()
            };
          }
        }
        
        cursor = res.cursor;
      } while (cursor);
      
      return { found: false };
    } catch (error) {
      console.error(`[storacha] Stat failed: ${error.message}`);
      return { found: false };
    }
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
    
    try {
      // Parse the root CID and remove the upload
      const rootCID = CID.parse(cid);
      const result = await client.capability.upload.remove(rootCID);
      
      if (result.error) {
        throw new Error(`Remove failed: ${result.error.message}`);
      }
      
      return { removed: true };
    } catch (error) {
      console.error(`[storacha] Remove failed for ${cid}: ${error.message}`);
      throw error;
    }
  },

  async copy({ cid, remote, size }) {
    if (!client) throw new Error('Client not initialized');
    if (!currentSpace) throw new Error('No space selected');
    
    // Server-side copy: CID already exists in IPFS, just return success
    // Storacha manages the uploads automatically
    return { cid: cid };
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
