 I want two binaries because then one can have all the dev dependencies and the other can be light and only for deploying onto a robot but

do we need dialtone-dev.go and dialtone.go? in the top of the directory ?
can dialtone.sh use dev.go directly ? 

research how to have just the shell file at the top of the directory and it can use src/dev.go

src/dialtone.go should not need any kind of wrapper unless it has to do with golang modules resolution or something? 

put your research here
