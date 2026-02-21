#!/usr/bin/env node
// storacha-bridge.mjs - Node.js bridge for rclone Storacha backend
// This script runs as a subprocess and communicates via stdin/stdout JSON

import * as Client from '@storacha/client';
import * as readline from 'readline';
import * as path from 'path';
import * as dagPB from '@ipld/dag-pb';
import { UnixFS } from 'ipfs-unixfs';
import { CID } from 'multiformats/cid';
import { sha256 } from 'multiformats/hashes/sha2';
import { CarWriter } from '@ipld/car';

let client = null;
let currentSpace = null;

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

  // ========================================
  // Content Mutation Operations
  // ========================================

  async fetchBlock({ cid }) {
    if (!client) throw new Error('Client not initialized');
    
    try {
      const res = await fetch(
        `https://ipfs.io/ipfs/${cid}?format=raw`,
        {
          headers: {
            'Accept': 'application/vnd.ipld.raw'
          }
        }
      );
      
      if (!res.ok) throw new Error(`Failed fetching block: ${res.status}`);
      
      const arrayBuffer = await res.arrayBuffer();
      const bytes = new Uint8Array(arrayBuffer);
      
      // Decode the DAG-PB node
      const node = dagPB.decode(bytes);
      
      const links = node.Links.map(l => ({
        name: l.Name,
        hash: l.Hash.toString(),
        size: l.Tsize || 0
      }));
      
      // Check if this is a directory by examining UnixFS data
      let isDir = false;
      try {
        const unixfs = UnixFS.unmarshal(node.Data);
        isDir = unixfs.type === 'directory';
      } catch (e) {
        // Not a UnixFS node, assume directory if has links
        isDir = links.length > 0;
      }
      
      return {
        cid: cid.toString(),
        data: node.Data,
        links: links,
        isDir: isDir,
        rawBytes: Buffer.from(bytes).toString('base64')
      };
    } catch (error) {
      throw new Error(`Failed to fetch block ${cid}: ${error.message}`);
    }
  },

  async createFileBlock({ content }) {
    try {
      const contentBytes = Buffer.isBuffer(content) 
        ? content 
        : Buffer.from(content, 'base64');
      
      const unixfs = new UnixFS({
        type: 'file',
        data: contentBytes
      });
      
      const node = dagPB.prepare({
        Data: unixfs.marshal(),
        Links: []
      });
      
      const bytes = dagPB.encode(node);
      const hash = await sha256.digest(bytes);
      const cid = CID.create(1, dagPB.code, hash);
      
      return {
        cid: cid.toString(),
        data: node.Data,
        links: [],
        isDir: false,
        rawBytes: Buffer.from(bytes).toString('base64')
      };
    } catch (error) {
      throw new Error(`Failed to create file block: ${error.message}`);
    }
  },

  async rebuildDirNode({ oldData, links }) {
    try {
      const linksArray = links.map(l => ({
        Name: l.name,
        Hash: CID.parse(l.hash),
        Tsize: l.size || 0
      }));
      
      const node = dagPB.prepare({
        Data: oldData instanceof Uint8Array ? oldData : Buffer.from(oldData, 'base64'),
        Links: linksArray
      });
      
      const bytes = dagPB.encode(node);
      const hash = await sha256.digest(bytes);
      const cid = CID.create(1, dagPB.code, hash);
      
      return {
        cid: cid.toString(),
        data: node.Data,
        links: links,
        isDir: true,
        rawBytes: Buffer.from(bytes).toString('base64')
      };
    } catch (error) {
      throw new Error(`Failed to rebuild directory node: ${error.message}`);
    }
  },

  async createCAR({ rootCID, blocks }) {
    try {
      const rootCidParsed = CID.parse(rootCID);
      const { writer, out } = CarWriter.create([rootCidParsed]);
      
      const chunks = [];
      const readerPromise = (async () => {
        for await (const chunk of out) {
          chunks.push(chunk);
        }
      })();
      
      // Add all blocks to the CAR
      for (const block of blocks) {
        const blockBytes = Buffer.from(block.rawBytes, 'base64');
        const blockCID = CID.parse(block.cid);
        await writer.put({ cid: blockCID, bytes: blockBytes });
      }
      
      await writer.close();
      await readerPromise;
      
      const carBytes = Buffer.concat(chunks);
      
      return {
        carBytes: carBytes.toString('base64'),
        size: carBytes.length
      };
    } catch (error) {
      throw new Error(`Failed to create CAR: ${error.message}`);
    }
  },

  async uploadCAR({ carData, rootCID }) {
    if (!client) throw new Error('Client not initialized');
    if (!currentSpace) throw new Error('No space selected');
    
    try {
      const carBytes = Buffer.from(carData, 'base64');
      const carBlob = new Blob([carBytes], { type: 'application/car' });
      
      let newShardCID = null;
      
      await client.uploadCAR(carBlob, {
        rootCID: CID.parse(rootCID),
        onShardStored: (meta) => {
          newShardCID = meta.cid.toString();
        }
      });
      
      if (!newShardCID) {
        throw new Error('Failed to get shard CID from upload');
      }
      
      return { shardCID: newShardCID };
    } catch (error) {
      throw new Error(`Failed to upload CAR: ${error.message}`);
    }
  },

  async getUploads({ rootCID }) {
    if (!client) throw new Error('Client not initialized');
    if (!currentSpace) throw new Error('No space selected');
    
    try {
      const targetRootCID = CID.parse(rootCID);
      const uploads = [];
      
      let cursor;
      do {
        const res = await client.capability.upload.list({ cursor });
        if (!res || !res.results) break;
        
        for (const upload of res.results) {
          if (upload.root.toString() === targetRootCID.toString()) {
            uploads.push({
              cid: upload.root.toString(),
              root: upload.root.toString(),
              shards: (upload.shards || []).map(s => s.cid.toString()),
              size: upload.shards?.reduce((sum, s) => sum + (s.size || 0), 0) || 0,
              time: upload.insertedAt || new Date().toISOString()
            });
          }
        }
        
        cursor = res.cursor;
      } while (cursor);
      
      return uploads;
    } catch (error) {
      console.error(`Failed to get uploads: ${error.message}`);
      return [];
    }
  },

  async registerShards({ rootCID, shards }) {
    if (!client) throw new Error('Client not initialized');
    if (!currentSpace) throw new Error('No space selected');
    
    try {
      const rootCidParsed = CID.parse(rootCID);
      const shardCIDs = shards.map(s => CID.parse(s));
      
      await client.capability.upload.add(rootCidParsed, shardCIDs);
      
      return { success: true };
    } catch (error) {
      throw new Error(`Failed to register shards: ${error.message}`);
    }
  },

  async getRootCID() {
    if (!client) throw new Error('Client not initialized');
    if (!currentSpace) throw new Error('No space selected');
    
    try {
      // Get the most recent upload's root CID
      const res = await client.capability.upload.list({ cursor: undefined });
      
      if (res && res.results && res.results.length > 0) {
        const latestUpload = res.results[0];
        return { rootCID: latestUpload.root.toString() };
      }
      
      throw new Error('No uploads found');
    } catch (error) {
      throw new Error(`Failed to get root CID: ${error.message}`);
    }
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
