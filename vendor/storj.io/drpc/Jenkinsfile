pipeline {
    agent {
        dockerfile {
            filename 'Dockerfile.jenkins'
            args '-u root:root --cap-add SYS_PTRACE -v "/tmp/gomod":/go/pkg/mod'
            label 'main'
        }
    }
    stages {
        stage('Download') {
            steps {
                checkout scm
                sh 'make download'
            }
        }
        stage('Test') {
            steps {
                sh 'make test'
            }
        }
        stage('Lint') {
            steps {
                sh 'make lint'
            }
        }
    }
}
