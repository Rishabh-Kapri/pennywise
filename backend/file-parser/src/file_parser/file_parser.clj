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

