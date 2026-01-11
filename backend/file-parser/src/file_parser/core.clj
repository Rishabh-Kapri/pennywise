(ns file-parser.core
  (:import (java.security MessageDigest))
  (:require
   [clojure.data.json :as json]
   [org.httpkit.server :as server]
   [file-parser.handler :as handler]
   [ring.middleware.defaults :refer [wrap-defaults site-defaults]])
  (:gen-class))

; def file upload status
; what do I need?
; hash attached to the user api key
; current status of the hash
; hash expires after 24 hours 
; do I need to store the processing logs as well?
; the data structure:
; {
;   "api-key": "hash-value"
; }
; {
;   "hash-value": { 
;     "status": "pending" | "failed" | "success" | "processing" | "expired",
;     "total" 0,
;     "processed": 0,
;     "created-at": "2017-01-01 00:00:00" ;; always return ISO 8601 string
;    }
; }

(def api-hash-store (atom {}))
(def hash-metadata-store (atom {}))

(defn sha256
  "Generates a sha256 of an input string.

  Arguments:
  - `input`: a string create the hash from

  Returns:
  - a sha256 hash of the input string
  "
  [input]
  (let [md (MessageDigest/getInstance "SHA-256")]
    (.update md (.getBytes input))
    (let [hash (.digest md)]
      (apply str (map #(format "%02x" %) hash)))))

(defn init-hash-metadata [hash-value]
  (let [metadata {:status "pending" :total 0 :processed 0}]
    (swap! hash-metadata-store assoc hash-value metadata)
    metadata))

(defn reset-map []
  (reset! api-hash-store {}))

(defn get-api-key-hash [api-key]
  (@api-hash-store api-key))

(defn create-new-hash
  "Creates a new hash for the api key, populates the hash metadata map and returns it
  
  Arguments:
  - `api-key` a string that is the api key to create a hash for
  if the api key is already in the map, returns the existing hash
  "
  [api-key]
  ; @TODO: handle error later
  ; (when-not (string? api-key)
  ;   (throw (IllegalArgumentException "API Key must be a string")))

  ;; if api key is not in the map, create a new hash and return it.
  (if (not (contains? @api-hash-store api-key))
    (let [hash (sha256 api-key)
          metadata {:status "pending", :total 0 :pending 0 :created-at ""}]
      (swap! api-hash-store assoc api-key hash)
      (swap! hash-metadata-store assoc hash metadata)
      hash)
    (get-api-key-hash api-key)))
  ;; we can simplify this by calling `or` which returns the first truthy value.

; (create-new-hash 1)
; (println (sha256  "1234abcd"))
; (println (@api-hash-store "1234abc"))
; (println @hash-metadata-store)
; (reset-map)

(defn -main [& args]
  (let [port (Integer/parseInt (or (System/getenv "PORT") "4000"))]
    (server/run-server
     (-> handler/app-routes
         (wrap-defaults (assoc-in site-defaults [:security :anti-forgery] false)))
     {:port port})
    (println (str "Webserver started at http://127.0.0.1:" port "/"))))
