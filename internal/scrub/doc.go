// Package scrub makes agent-written strings safe to store: it redacts likely
// secrets (API keys, tokens, private keys, passwords, credentials in URLs)
// and bounds text to one line of a given length (SEC-3, HOOK-2).
//
// It is pure. Deciding which fields of a hook payload may be stored at all
// belongs to the hook adapters and events; scrub only cleans what they keep.
package scrub
