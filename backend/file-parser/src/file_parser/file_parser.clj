(ns file-parser.file-parser
  (:gen-class))

(def data [1,10,3,4])

(defn square [x] (* x x))

;; apply the square function to each element of data
; (map square data)

(defn process [nums]
  (->> nums
       (filter odd?)
       (map square)))

(process data)

(defn double [x]
  (* x 3))

(comment
  (double 5))

(defn messenger
  ([] (messenger "Hello world!"))
  ([msg] (println msg)))

(messenger)

(defn plotxy [shape x y]
  (println shape)
  (println "x: " x " y: " y))

(defn plot [shape coords]
  (apply plotxy  shape coords))

(plot "circle" [1 2])

(defn messenger-builder [greeting]
  (fn [who] (println greeting who)))

(def hello-er (messenger-builder "Hello"))

(hello-er "world!")

(def scores {"Fred" 1000, "Alice" 200})

(assoc scores "Alice" 1000) ;; {"Fred" 1000, "Alice" 1000} returns a new map
(assoc scores "Sue" 100)
print scores ;; {"Fred" 1000, "ALice" 200} the original map is not mutated
