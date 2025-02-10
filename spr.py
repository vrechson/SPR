import requests
import sys
import json
import re
import threading
from urllib3.exceptions import InsecureRequestWarning
from urllib.parse import unquote
from queue import Queue

requests.packages.urllib3.disable_warnings(InsecureRequestWarning)

semaphore = threading.Semaphore(10)
queue = Queue()

def returnTextFromTypeParams(paramDict):
    if 'type' in paramDict['schema']:
        if paramDict['schema']['type'] == 'string':
            return '{}=arthur&'.format(paramDict['name'])
        elif paramDict['schema']['type'] == 'integer':
            return '{}=123&'.format(paramDict['name'])
        elif paramDict['schema']['type'] == 'boolean':
            return '{}=False&'.format(paramDict['name'])
    return 'None'

def send_request(method, url):
    with semaphore:
        try:
            response = requests.request(method.upper(), url, verify=False, proxies={"http":"http://127.0.0.1:8080","https":"http://127.0.0.1:8080"})
            print('{} - {}'.format(url, response.status_code))
        except Exception as e:
            print(f'Erro ao acessar {url}: {e}')

def worker():
    while True:
        task = queue.get()
        if task is None:
            break
        method, url = task
        send_request(method, url)
        queue.task_done()

def main():
    try:
        swaggerFile = sys.argv[1]
        urlHost = sys.argv[2]
    except:
        print("Adicione uma URL/Arquivo válido.")
        exit()

    jsonFile = json.load(open(swaggerFile, 'r'))

    threads = []
    for _ in range(10):
        t = threading.Thread(target=worker)
        t.daemon = True
        t.start()
        threads.append(t)

    for apiEndpoints in jsonFile['paths']:
        completeUrl = urlHost + apiEndpoints

        for methodAndRequest in jsonFile['paths'][apiEndpoints]:
            finalUrl = ""
            
            if methodAndRequest.upper() == "GET":
                urlParams = "?"
                if 'parameters' in jsonFile['paths'][apiEndpoints][methodAndRequest]:
                    for params in jsonFile['paths'][apiEndpoints][methodAndRequest]['parameters']:
                        if params['in'] == 'query':
                            urlParams += returnTextFromTypeParams(params)
                finalUrl = '{}{}{}'.format(urlHost, apiEndpoints, urlParams[:-1])
            else:
                continue  
            
            finalUrl = unquote(finalUrl)
            if '{' in finalUrl or '}' in finalUrl:
                padrao = r'\{[^}]+\}'
                finalUrl = re.sub(padrao, '123', finalUrl)
            
            queue.put((methodAndRequest.upper(), finalUrl))
    
    queue.join()
    for _ in range(10):
        queue.put(None)
    for t in threads:
        t.join()

if __name__ == "__main__":
    main()
