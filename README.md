# Synapticz Backend

Synapticz is a learning platform designed to help users reinforce knowledge through quizzes and other learning tools. This is the backend, built using **Golang**, **Fiber**, and **PostgreSQL**.

## Features

- **User Authentication** (Login/Signup)
- **Quiz Management** (Create, Edit, Delete Questions)
- **Performance Tracking**
- **Future Enhancements**: Flashcards, Spaced Repetition, and more

## Tech Stack

- **Backend:** Golang (Fiber Framework)
- **Database:** PostgreSQL
- **ORM:** Raw SQL (No GORM, for better learning and control)
- **Authentication:** JWT-based authentication
- **Deployment:** Docker & CI/CD (Planned)

## Getting Started

### Prerequisites

Ensure you have the following installed:

- [Go](https://go.dev/doc/install) (>= 1.18)
- [PostgreSQL](https://www.postgresql.org/download/)
- [Git](https://git-scm.com/)

### Installation

Clone the repository:

```sh
git clone https://github.com/ShijuPJohn/synapticz_backend.git
cd synapticz_backend
```

Initialize the Go module:

```sh
go mod init github.com/ShijuPJohn/synapticz_backend
go mod tidy
```

### Environment Setup

Create a `.env` file and add your database credentials:

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=youruser
DB_PASSWORD=yourpassword
DB_NAME=synapticz_db
JWT_SECRET=your_secret_key
```

### Running the Server

```sh
go run main.go
```

The API should now be running on `http://localhost:3000`


## Contributing

1. Fork the repo
2. Create a new branch (`git checkout -b feature-xyz`)
3. Commit changes (`git commit -m 'Add feature XYZ'`)
4. Push to your branch (`git push origin feature-xyz`)
5. Open a pull request

## License

This project is licensed under the MIT License.