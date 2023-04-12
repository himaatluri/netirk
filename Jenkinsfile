node { 
    stage('Git') {
        git 'https://github.com/himasagaratluri/netirk.git'
    }

    stage('Go build') {
        sh 'go build main.go'
    }
    stage('Fossa Analyze') {
        sh 'curl -H \'Cache-Control: no-cache\' https://raw.githubusercontent.com/fossas/fossa-cli/master/install-latest.sh | bash'
        sh 'FOSSA_API_KEY=XXXXXXXXXXXXXXXXXXXX fossa analyze'
    }
}