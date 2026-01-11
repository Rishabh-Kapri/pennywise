(ns file-parser.handler
  (:import (java.security MessageDigest))
  (:require [compojure.route :as route]
            [clojure.data.json :as json]
            [compojure.core :refer [defroutes GET POST PUT DELETE]])
  (:gen-class))

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

(defn get-x-api-key
  "Get the API Key from the request header.

  Arguments:
  - req - the request object.

  Returns:
  - the API Key.
  - throws an exception if the API Key is missing.
  "
  [req]
  (let [api-key (get (-> req :headers) "x-api-key")]
    (if (not api-key)
      (throw (ex-info "No API Key" {:status 401}))
      api-key)))

(defn apikey-handler [req]
  (try
    (let [api-key (get-x-api-key req)]
      (println (str "API Key: " api-key))
      {:status 200
       :headers {"Content-Type", "text/plain"}
       :body  (json/write-str api-key)})
    (catch clojure.lang.ExceptionInfo e
      (let [{:keys [status]} (ex-data e)]
        {:status (or status 500)
         :headers {"Content-Type", "application/json"}
         :body (json/write-str {:error (.getMessage e)})}))
    (catch Exception e
      {:status 500
       :headers {"Content-Type", "application/json"}
       :body (json/write-str  {:error (.getMessage e)})})))

(defn createapikey-handler [req]
  (println "Create API Key Handler"))

(def mock-request {:headers {}})

(apikey-handler mock-request)

(defroutes app-routes
  (GET "/api-key", [] apikey-handler)
  (POST "/api-key/create", [] createapikey-handler)
  (route/not-found "Page not found!"))
