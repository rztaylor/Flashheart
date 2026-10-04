// Package webui embeds and serves Flashheart's compiled browser frontend.
//
// It owns static-asset availability and cache policy for the files Vite writes
// to assets/generated. Frontend source lives under frontend/; routing,
// security headers and the Singleserve lifecycle belong to app, and API
// routes to api.
package webui
