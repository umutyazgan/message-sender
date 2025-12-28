module.exports = {
    environments: {
        $shared: {
            "host": "http://localhost:8080",
        },
        $default: {
            user: "mario",
            password: "123456",
        },
        dev: {
            "user": "mario",
            "password": "123456",
        },
        prod: {
            "user": "mario",
            "password": "password$ecure123",
        }
    }
}