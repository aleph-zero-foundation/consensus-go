'''Routines called by fab. Assumes that all are called from */experiments/aws.'''

from fabric import task


#======================================================================================
#                                    installation
#======================================================================================

@task
def setup(conn):
    ''' Install dependencies in a nonblocking way.'''

    conn.put('setup.sh', '.')
    conn.sudo('apt update', hide='both')
    conn.sudo('apt install dtach', hide='both')
    conn.run('dtach -n `mktemp -u /tmp/dtach.XXXX` bash setup.sh', hide='both')

@task
def setup_completed(conn):
    ''' Check if installation completed.'''

    result = conn.run('tail -1 setup.log')
    return result.stdout.strip()

@task
def clone_repo(conn):
    '''Clones main repo, checkouts to devel, and installs it via pip.'''

    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    # delete current repo
    conn.run(f'rm -rf {repo_path}')
    # clone using deployment token
    user_token = 'gitlab+deploy-token-70309:G2jUsynd3TQqsvVfn4T7'
    conn.run(f'git clone http://{user_token}@gitlab.com/alephledger/consensus-go.git {repo_path}')

@task
def build_gomel(conn):
    conn.run(f'PATH="$PATH:/snap/bin" && go build /home/ubuntu/go/src/gitlab.com/alephledger/consensus-go/cmd/gomel')

@task
def inst_deps(conn):
    conn.put('deps.sh', '.')
    conn.run('PATH="$PATH:/snap/bin" && dtach -n `mktemp -u /tmp/dtach.XXXX` bash deps.sh', hide='both')

#======================================================================================
#                                   syncing local version
#======================================================================================

@task 
def send_data(conn, pid):
    ''' Sends keys, addresses, and parameters. '''
    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    conn.put(f'data/{pid}.keys', repo_path)

    # TODO send parameters

#======================================================================================
#                                   run experiments
#======================================================================================

@task
def run_protocol(conn, pid):
    ''' Runs the protocol.'''

    repo_path = '/home/ubuntu/go/src/gitlab.com/alephledger/consensus-go'
    with conn.cd(repo_path):
        cmd = f'go run cmd/gomel/main.go --keys {pid}.keys --db pkg/testdata/users.txt --log {pid}.log'
        conn.run(f'PATH="$PATH:/snap/bin" && dtach -n `mktemp -u /tmp/dtach.XXXX` {cmd}')

@task
def get_logs(conn):
    ''' Retrieves aleph.log from the server.'''

    ip = conn.host.replace('.', '-')

    with conn.cd('proof-of-concept/aleph/'):
        conn.run(f'zip -q {ip}-aleph.log.zip aleph.log')
    conn.get(f'proof-of-concept/aleph/{ip}-aleph.log.zip', f'../results/{ip}-aleph.log.zip')


@task
def stop_world(conn):
    ''' Kills the committee member.'''

    # it is safe as python refers to venv version
    conn.run('killall python')


@task
def version(conn):
    ''' Always changing task for experimenting with fab.'''

    conn.run(f'PATH="$PATH:/snap/bin" && go version')
