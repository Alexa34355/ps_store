package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"text/template"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var database *sql.DB

func main() {
	db, err := sql.Open("mysql", "root:root@/store_ps")

	if err != nil {
		log.Println(err)
	}
	database = db
	defer db.Close()

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("/", Index)
	http.HandleFunc("/add_product", AddProduct)
	http.HandleFunc("/register", Register)
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/logout", LogoutHandler)
	http.HandleFunc("/EditItem", EditProduct)
	http.HandleFunc("/Cart", Cartt)
	fmt.Println("Server is listening...")
	http.ListenAndServe("127.0.0.1:1001", nil)
}

func Cartt(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {

		removeItemFromCart := r.URL.Query().Get("deleteProductFromCart")
		if removeItemFromCart != "" {

			id, _ := strconv.Atoi(removeItemFromCart)
			proudct := FindProductInDB(id)

			err := RemoveItemFromCartInDB(user.ID, proudct.ID)
			if err != nil {
				fmt.Printf("Delete item from DB: %s", err)
			}

			Redirect(w, r, "/Cart")
		}
		type Data struct {
			MyCart    []Cart
			TotalCost float64
		}
		var data Data
		cart, _ := GetCart(user.ID)
		fmt.Print(len(cart))
		data.MyCart = cart
		for i := 0; i < len(data.MyCart); i++ {
			data.TotalCost += data.MyCart[i].Cost
		}
		temp, _ := template.ParseFiles("static/html/trash.html")
		temp.Execute(w, data)

	}

}

func RemoveItemFromCartInDB(userID, productID int) error {

	if userID <= 0 || productID <= 0 {
		return errors.New("UserID or ProductID <= 0")
	}

	query := "DELETE FROM Cart WHERE UserID=? AND ProductID=?"
	_, err := database.Exec(query, userID, productID)

	if err != nil {
		return err
	}

	return nil
}

func GetCart(userID int) ([]Cart, error) {

	query := "SELECT ProductID FROM Cart WHERE UserID=?"
	rows, err := database.Query(query, userID)
	cart := []Cart{}

	if err != nil {
		log.Println(err)
	}
	defer rows.Close()

	for rows.Next() {
		c := Cart{}
		cartDB := CartDB{}
		product := Product{}

		err := rows.Scan(&cartDB.ProductID)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		queryProduct := "SELECT ID, Name, Cost FROM Products WHERE ID=?"
		row := database.QueryRow(queryProduct, cartDB.ProductID)
		errProduct := row.Scan(&product.ID, &product.Name, &product.Cost)

		if errProduct != nil {
			fmt.Println(err.Error())
			continue
		}

		c.ID = product.ID
		c.Name = product.Name
		c.Cost = product.Cost

		cart = append(cart, c)
	}

	return cart, nil
}

func AddToCartInDB(userID, productID int) error {

	query := "INSERT INTO Cart (UserID, ProductID) VALUES (?, ?)"
	_, err := database.Exec(query, userID, productID)

	return err
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {

	SetCookie(w, "")
	Redirect(w, r, "/")
}

func Redirect(w http.ResponseWriter, r *http.Request, path string) {

	http.Redirect(w, r, path, http.StatusSeeOther)
}

var errorMessage string

func Register(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			fmt.Fprintf(w, "ParseForm() err: %v", err)
			return
		}

		login := r.FormValue("login")
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirmPassword")

		if password != confirmPassword {
			fmt.Fprintln(w, "Password != Confir Password")
			return
		}

		hashPassword, err := GetMD5Hash(password)
		if err != nil {
			log.Fatalf(err.Error())
		}

		isUser, message := AddUserInDB(login, hashPassword)
		fmt.Print(isUser)
		fmt.Print(message)
		if isUser {
			errorMessage = ""

			SetCookie(w, login)
			Redirect(w, r, "/")
		} else {
			errorMessage = message
			Redirect(w, r, "singUp")
		}

	} else {
		temp, _ := template.ParseFiles("static/html/Register.html")
		temp.Execute(w, errorMessage)
		// http.ServeFile(w, r, "./html/singUp.html")
	}
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			fmt.Fprintf(w, "ParseForm() err: %v", err)
			return
		}

		login := r.FormValue("login")
		password, err := GetMD5Hash(r.FormValue("password"))
		if err != nil {
			log.Fatalf(err.Error())
		}

		// if len(login) < 6 || len(password) < 6 {
		//  errorMessage = "Login or Password is short"
		//  return
		// } else if len(login) > 20 || len(password) > 20 {
		//  errorMessage = "Login or Password is long"
		//  return
		// }

		isUser, message := FindUserInDB(login, password)

		if isUser {
			errorMessage = ""
			SetCookie(w, login)
			Redirect(w, r, "/")
		} else {
			errorMessage = message
			Redirect(w, r, "login")
		}
	} else {
		temp, _ := template.ParseFiles("static/html/login.html")
		temp.Execute(w, errorMessage)

		//http.ServeFile(w, r, "./html/login.html")
	}
}

func EditProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var product Product

		if err := r.ParseForm(); err != nil {
			fmt.Fprintf(w, "ParseForm() err: %v", err)
			return
		}

		r.ParseMultipartForm(0)
		product.ID, _ = strconv.Atoi(r.FormValue("ID"))
		product.Name = r.FormValue("name")
		product.Description = r.FormValue("description")
		product.Cost, _ = strconv.ParseFloat(r.FormValue("cost"), 64)
		product.Category = r.Form["category"][0]
		product.IMG = ""

		file, fileHeader, err := r.FormFile("img")
		if err == nil {
			defer file.Close()

			err = os.MkdirAll("./static/img", os.ModePerm)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			imageFileName := fmt.Sprintf("%d%s", time.Now().UnixNano(), filepath.Ext(fileHeader.Filename))
			dst, err := os.Create(fmt.Sprintf("./static/img/%s", imageFileName))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			defer dst.Close()

			_, err = io.Copy(dst, file)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			product.IMG = imageFileName
		}

		EditItem(product)
		Redirect(w, r, "/")
	} else {
		tmpl, _ := template.ParseFiles("static/html/editItem.html")
		tmpl.Execute(w, product)
	}
}

func EditItem(product Product) (bool, error) {

	if product.ID <= 0 {
		return false, errors.New("Item ID not found")
	}

	if product.IMG == "" {
		query := "UPDATE Products SET Name=?, Description=?, Category=?, Cost=? WHERE ID=?"
		_, err := database.Exec(query, product.Name, product.Description, product.Category, product.Cost, product.ID)

		if err != nil {
			fmt.Println(err.Error())
			return false, err
		}
	} else {
		query := "UPDATE Products SET Name=?, Description=?, Category=?, img=?, Cost=? WHERE ID=?"
		_, err := database.Exec(query, product.Name, product.Description, product.Category, product.IMG, product.Cost, product.ID)

		if err != nil {
			fmt.Println(err.Error())
			return false, err
		}
	}

	return true, nil
}

func FindUserInDB(login, passwordHash string) (bool, string) {

	var user User
	query := "SELECT Login, Password FROM Users WHERE Login=?"

	row := database.QueryRow(query, login)
	err := row.Scan(&user.Login, &user.Password)

	return err == nil, ""
}

func AddUserInDB(login, hashPassword string) (bool, string) {

	// queryGetUser := "SELECT Login FROM Users WHERE Login=?"
	// _, errGetUser := database.Exec(queryGetUser, login)

	// if errGetUser == nil
	queryInsertUser := "INSERT INTO Users (Login, Password) VALUES (?, ?)"
	_, err := database.Exec(queryInsertUser, login, hashPassword)

	if err != nil {
		log.Println(err)
	}

	return true, ""

	return false, "User with this Login '" + login + "' alredy registered"
}

func GetCookie(w http.ResponseWriter, r *http.Request) (string, error) {

	loginCookie, err := r.Cookie("Login")
	if err != nil {
		return "", err
	}

	return loginCookie.Value, err
}

func SetCookie(w http.ResponseWriter, login string) {

	loginCookie := &http.Cookie{
		Name:    "Login",
		Value:   login,
		Expires: time.Now().Add(365 * 24 * time.Hour),
	}

	http.SetCookie(w, loginCookie)
}

func GetMD5Hash(text string) (string, error) {

	if len(text) == 0 {
		return "", errors.New("Input text null")
	}

	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:]), nil
}

func AddProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			fmt.Fprintf(w, "ParseForm() err: %v", err)
			return
		}

		r.ParseMultipartForm(0)

		name := r.FormValue("name")
		description := r.FormValue("description")
		cost := r.FormValue("cost")
		category := r.Form["category"][0]

		file, fileHeader, err := r.FormFile("img")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		defer file.Close()

		err = os.MkdirAll("./static/img", os.ModePerm)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		imageFileName := fmt.Sprintf("%d%s", time.Now().UnixNano(), filepath.Ext(fileHeader.Filename))
		dst, err := os.Create(fmt.Sprintf("./static/img/%s", imageFileName))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		defer dst.Close()

		_, err = io.Copy(dst, file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s, _ := strconv.ParseFloat(cost, 64)

		var product Product
		product.Name = name
		product.Description = description
		product.Cost = s
		product.IMG = imageFileName
		product.Category = category

		AddItemInDB(product)
		Redirect(w, r, "/")
	} else {
		http.ServeFile(w, r, "static/html/addItem.html")
	}
}

func GetProducts() []Product {
	rows, err := database.Query("SELECT * FROM Products")
	if err != nil {
		log.Println(err)
	}
	defer rows.Close()
	products := []Product{}

	for rows.Next() {
		p := Product{}
		err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Category, &p.IMG, &p.Cost)
		if err != nil {
			fmt.Println(err)
			continue
		}
		products = append(products, p)
	}

	return products
}

func AddItemInDB(product Product) bool {

	query := "INSERT INTO Products (Name, Description, Category, img, Cost) VALUES (?, ?, ?, ?, ?)"
	_, err := database.Exec(query, product.Name, product.Description, product.Category, product.IMG, product.Cost)

	if err != nil {
		log.Println(err)
	}

	return err != nil
}

func FindProductInDB(id int) Product {

	var product Product
	query := "SELECT * FROM Products WHERE ID=?"

	row := database.QueryRow(query, id)
	err := row.Scan(&product.ID, &product.Name, &product.Description, &product.Category, &product.IMG, &product.Cost)

	if err != nil {
		fmt.Println(err.Error())
	}

	return product
}

var user User
var product Product
var cart []Cart

func Index(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" {
		var data ViewData
		products := GetProducts()

		data.Products = products

		edit := r.URL.Query().Get("edit")
		if edit != "" {
			id, _ := strconv.Atoi(edit)
			product = FindProductInDB(id)
			Redirect(w, r, "EditItem")
		}

		cookie, errGetCookie := GetCookie(w, r)
		fmt.Print(cookie)
		user, _ = GetUserFromDB(cookie)

		if errGetCookie != nil {
			fmt.Printf("Cookie: %s", errGetCookie)
		}

		removeProduct := r.URL.Query().Get("removeProduct")
		addCard := r.URL.Query().Get("cart")
		if removeProduct != "" {
			id, _ := strconv.Atoi(removeProduct)
			DeleteProductFromDB(id)
			Redirect(w, r, "/")
		} else if addCard != "" {
			id, _ := strconv.Atoi(addCard)
			AddToCartInDB(user.ID, id)
			Redirect(w, r, "/")
		}

		if cookie != "" {

			if user.Privilege == 1 {
				data.IsPrivilege = true
			}

			data.UserName = user.Login
			data.IsLogin = true

		}

		tmpl, _ := template.ParseFiles("main.html")
		tmpl.Execute(w, data)
	}
}

func GetUserFromDB(login string) (User, error) {

	if len(login) == 0 {
		return User{}, nil
	}

	var user User
	query := "SELECT ID, Login, Password, Privilege FROM Users WHERE Login=?"

	row := database.QueryRow(query, login)
	err := row.Scan(&user.ID, &user.Login, &user.Password, &user.Privilege)

	if err != nil {
		log.Print(err)
	}

	// if err != nil {
	//  fmt.Println(err)
	//  return User{}, err
	// }

	return user, nil
}

func DeleteProductFromDB(ID int) {

	// errCart := RemoveItemsFromCartInDB(ID)

	query := "DELETE FROM Products WHERE ID=?"
	_, err := database.Exec(query, ID)

	// if errCart != nil {
	//  fmt.Println(errCart.Error())
	// }

	if err != nil {
		log.Fatalln(err.Error())
	}
}

type ViewData struct {
	IsPrivilege bool
	IsLogin     bool
	UserName    string
	Products    []Product
}

type Cart struct {
	ID   int
	Name string
	Cost float64
}

type CartDB struct {
	ID        int
	UserID    int
	ProductID int
}

type Order struct {
	UserID    int
	ProductID int
}

type Product struct {
	ID          int
	Name        string
	Description string
	Category    string
	IMG         string
	Cost        float64
}

type User struct {
	ID        int
	Login     string
	Password  string
	Privilege int
}
