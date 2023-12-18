pipeline {
  agent any

  stages {
    stage("Golang linter"){
        steps{
               sh "golangci-lint run"
        }
    }
    stage('Make Test') {
      steps{
        sh "make test"
      }
    }
  }
}