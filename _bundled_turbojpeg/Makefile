
all: libturbojpeg.a

libturbojpeg.a: Dockerfile
	docker build --rm -t libturbojpeg .
	docker run --rm -v $$(pwd)/:/tmp/gokrazy/ libturbojpeg sh -c 'cp /usr/lib/aarch64-linux-gnu/libturbojpeg.a /usr/include/turbojpeg.h /usr/include/aarch64-linux-gnu/jconfig.h /usr/include/jerror.h /usr/include/jmorecfg.h /usr/include/jpegint.h /usr/include/jpeglib.h /usr/lib/aarch64-linux-gnu/libjpeg.a /tmp/gokrazy/'

clean:
	rm -f jconfig.h jerror.h jmorecfg.h jpegint.h jpeglib.h libjpeg.a libturbojpeg.a turbojpeg.h 

