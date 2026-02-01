use https://github.com/timcash/code_cad as an example
2. create a new plugin for dialtone that has a golang file to install and build it
3. make sure the plugin has a test comand 
4. call the plugin `cad` 
5. it should run the backend with the command `./dialtone.sh cad server`
6. the fontend should be integrated into the `www dev` website as a `<section>` called `cad`
7. is should have a compontent `cad.ts` file 
8. create a new `./plugin ticket` to track your work
9. make a git branch called `www-cad-plugin` 
10. make it so the backend can make the CAD object and the frontend can display it
11. integrate it with the `www` pages styles and `<section>`