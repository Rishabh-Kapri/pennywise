(ns file-parser.test
  (:gen-class))

(defn hello-world []
  (.exists skk "file-parser.txt")
  (println "hello world"))

(hello-world)

