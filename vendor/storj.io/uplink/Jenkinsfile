pipeline {
    agent none

    options {
          timeout(time: 26, unit: 'MINUTES')
    }
    environment {
        NPM_CONFIG_CACHE = '/tmp/npm/cache'
        COCKROACH_MEMPROF_INTERVAL=0
    }
    stages {
        stage('Build') {
            agent {
                docker {
                    label 'main'
                    image 'storjlabs/ci:latest'
                    alwaysPull true
                    args '-u root:root --cap-add SYS_PTRACE -v "/tmp/gomod":/go/pkg/mod'
                }
            }
            stages {
                stage('Preparation') {
                    parallel {
                        stage('Checkout') {
                            steps {
                                checkout scm

                                sh 'mkdir -p .build'
                                // make a backup of the mod file in case, for later linting
                                sh 'cp go.mod .build/go.mod.orig'
                                sh 'cp testsuite/go.mod .build/testsuite.go.mod.orig'
                            }
                        }
                        stage('Start databases') {
                            steps {
                                sh 'service postgresql start'

                                dir('.build') {
                                    sh 'cockroach start-single-node --insecure --store=\'/tmp/crdb\' --listen-addr=localhost:26257 --http-addr=localhost:8080 --cache 512MiB --max-sql-memory 512MiB --background'
                                }
                            }
                        }
                    }
                }

                stage('Verification') {
                    parallel {
                        stage('Lint') {
                            steps {
                                sh 'check-copyright'
                                sh 'check-large-files'
                                sh 'check-imports ./...'
                                sh 'check-peer-constraints'
                                sh 'storj-protobuf --protoc=$HOME/protoc/bin/protoc lint'
                                sh 'storj-protobuf --protoc=$HOME/protoc/bin/protoc check-lock'
                                sh 'check-atomic-align ./...'
                                sh 'check-monkit ./...'
                                sh 'check-errs ./...'
                                sh './scripts/check-dependencies.sh'
                                sh 'staticcheck ./...'
                                sh 'golangci-lint --config /go/ci/.golangci.yml -j=2 run'
                                sh 'go-licenses check ./...'
                                sh './scripts/check-libuplink-size.sh'
                                sh 'check-mod-tidy -mod .build/go.mod.orig'

                                dir("testsuite") {
                                    sh 'check-imports ./...'
                                    sh 'check-atomic-align ./...'
                                    sh 'check-monkit ./...'
                                    sh 'check-errs ./...'
                                    sh 'staticcheck ./...'
                                    sh 'golangci-lint --config /go/ci/.golangci.yml -j=2 run'
                                    sh 'check-mod-tidy -mod ../.build/testsuite.go.mod.orig'
                                }
                            }
                        }

                        stage('Tests') {
                            environment {
                                COVERFLAGS = "${ env.BRANCH_NAME == 'main' ? '-coverprofile=.build/coverprofile -coverpkg=./...' : ''}"
                            }
                            steps {
                                sh 'go vet ./...'
                                sh 'go test -parallel 4 -p 6 -vet=off $COVERFLAGS -timeout 20m -json -race ./... > .build/tests.json'
                            }

                            post {
                                always {
                                    sh script: 'cat .build/tests.json | xunit -out .build/tests.xml', returnStatus: true
                                    sh script: 'cat .build/tests.json | tparse -all -top -slow 100', returnStatus: true
                                    archiveArtifacts artifacts: '.build/tests.json'
                                    junit '.build/tests.xml'

                                    script {
                                        if(fileExists(".build/coverprofile")){
                                            sh script: 'filter-cover-profile < .build/coverprofile > .build/clean.coverprofile', returnStatus: true
                                            sh script: 'gocov convert .build/clean.coverprofile > .build/cover.json', returnStatus: true
                                            sh script: 'gocov-xml  < .build/cover.json > .build/cobertura.xml', returnStatus: true
                                            cobertura coberturaReportFile: '.build/cobertura.xml'
                                        }
                                    }
                                }
                            }
                        }

                        stage('Testsuite') {
                            environment {
                                STORJ_TEST_COCKROACH = 'cockroach://root@localhost:26257/testcockroach?sslmode=disable'
                                STORJ_TEST_POSTGRES = 'postgres://postgres@localhost/teststorj?sslmode=disable'
                                STORJ_TEST_COCKROACH_NODROP = 'true'
                                STORJ_TEST_LOG_LEVEL = 'info'
                                COVERFLAGS = "${ env.BRANCH_NAME == 'main' ? '-coverprofile=../.build/testsuite_coverprofile -coverpkg=storj.io/uplink/...' : ''}"
                            }
                            steps {
                                sh 'cockroach sql --insecure --host=localhost:26257 -e \'create database testcockroach;\''
                                sh 'psql -U postgres -c \'create database teststorj;\''
                                dir('testsuite'){
                                    sh 'go vet ./...'
                                    sh 'go test -parallel 4 -p 6 -vet=off $COVERFLAGS -timeout 20m -json -race ./... > ../.build/testsuite.json'
                                }
                            }

                            post {
                                always {
                                    dir('testsuite'){
                                        sh script: 'cat ../.build/testsuite.json | xunit -out ../.build/testsuite.xml', returnStatus: true
                                    }
                                    sh script: 'cat .build/testsuite.json | tparse -all -top -slow 100', returnStatus: true
                                    archiveArtifacts artifacts: '.build/testsuite.json'
                                    junit '.build/testsuite.xml'

                                    script {
                                        if(fileExists(".build/testsuite_coverprofile")){
                                            sh script: 'filter-cover-profile < .build/testsuite_coverprofile > .build/clean.testsuite_coverprofile', returnStatus: true
                                            sh script: 'gocov convert .build/clean.testsuite_coverprofile > .build/testsuite_cover.json', returnStatus: true
                                            sh script: 'gocov-xml  < .build/testsuite_cover.json > .build/testsuite_cobertura.xml', returnStatus: true
                                            cobertura coberturaReportFile: '.build/testsuite_cobertura.xml'
                                        }
                                    }
                                }
                            }
                        }

                        stage('Integration [storj/storj]') {
                            environment {
                                STORJ_TEST_POSTGRES = 'postgres://postgres@localhost/teststorj2?sslmode=disable'
                                STORJ_TEST_COCKROACH = 'omit'
                                // TODO add 'omit' for metabase STORJ_TEST_DATABASES
                                STORJ_TEST_DATABASES = 'pg|pgx|postgres://postgres@localhost/testmetabase?sslmode=disable'
                            }
                            steps {
                                sh 'psql -U postgres -c \'create database teststorj2;\''
                                sh 'psql -U postgres -c \'create database testmetabase;\''
                                dir('testsuite'){
                                    sh 'cp go.mod go-temp.mod'
                                    sh 'go vet -modfile go-temp.mod -mod=mod storj.io/storj/...'
                                    sh 'go test -modfile go-temp.mod -mod=mod -tags noembed -parallel 4 -p 6 -vet=off -timeout 20m -json storj.io/storj/... 2>&1 | tee ../.build/testsuite-storj.json | xunit -out ../.build/testsuite-storj.xml'
                                }
                            }

                            post {
                                always {
                                    sh script: 'cat .build/testsuite-storj.json | tparse -all -top -slow 100', returnStatus: true
                                    archiveArtifacts artifacts: '.build/testsuite-storj.json'
                                    junit '.build/testsuite-storj.xml'
                                }
                            }
                        }
                        stage('Go Compatibility') {
                            steps {
                                sh 'check-cross-compile -compiler "go,go.min" storj.io/uplink/...'
                            }
                        }
                    }
                }
            }
            post {
                always {
                    sh "chmod -R 777 ." // ensure Jenkins agent can delete the working directory
                    deleteDir()
                }
            }
        }

        stage('Integration [rclone]') {
            agent {
                node {
                    label 'ondemand'
                }
            }
            steps {
                    echo 'Testing rclone integration'
                    sh './testsuite/scripts/rclone.sh'
            }
            post {
                always {
                    zip zipFile: 'rclone-integration-tests.zip', archive: true, dir: '.build/rclone-integration-tests'
                    sh "chmod -R 777 ." // ensure Jenkins agent can delete the working directory
                    deleteDir()
                }
            }
        }
    }
}
