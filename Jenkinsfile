node { 
    stage('Git') {
        git 'https://github.com/himasagaratluri/netirk.git'
    }

    withCredentials([usernamePassword(credentialsId: 'FOSSA_API_KEY', passwordVariable: 'APITOKEN')]) {
        sh 'curl -H \'Cache-Control: no-cache\' https://raw.githubusercontent.com/fossas/fossa-cli/master/install-latest.sh | bash'
        sh 'FOSSA_API_KEY=$APITOKEN fossa analyze'
    }
}