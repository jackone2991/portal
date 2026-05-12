<!-- <!DOCTYPE html>
<html>

<head>
    <title>Parcel Sandbox</title>
    <meta charset="UTF-8" />
    <meta name="csrf-token" content="{{ csrf_token() }}">
    <script src="https://cdnjs.cloudflare.com/ajax/libs/jquery/3.6.0/jquery.js"></script>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0-alpha3/dist/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-KK94CHFLLe+nY2dmCWGMq91rCGa5gtU4mk92HdvYe+M/SXH301p5ILy+dN9+nJOZ" crossorigin="anonymous">
    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0-alpha3/dist/js/bootstrap.bundle.min.js" integrity="sha384-ENjdO4Dr2bkBIFxQpeoTz1HIcje39Wm4jDKdf19U8gI4ddQ3GYNS7NTKfAdVQSZe" crossorigin="anonymous"></script>
</head>
<style>
    body {
        font-family: sans-serif;
    }

    img {
        width: 100%;
        max-width: 50%;
        display: block;
    }
</style>

<body>
                <form action="/upload" method="POST" enctype="multipart/form-data">
                    @CSRF
                    <div class="br"></div>
                    <div class="br"></div>
                    <input type="file" class="form-control" name="thing">
                    <div class="br"></div>
                    <input type="submit" name="upload" class="btn btn-sm btn-block btn-danger" value="Upload" >
                </form>

                


</body>

</html> -->


<!DOCTYPE html>
<html>
<head>
    <title>CKEditor Image Upload Example</title>
    <meta name="csrf-token" content="{{ csrf_token() }}">
    <script src="https://cdn.ckeditor.com/ckeditor5/38.1.1/classic/ckeditor.js"></script>
    <script src="{{ asset('ckfinder/ckfinder.js') }}"></script>
    <!-- <script src="{{ asset('v1/js/ckeditor5_38.1.1_classic_ckeditor.js') }}"></script> -->
</head>
<body>
    <h1>CKEditor Image Upload Example - LaravelTuts.com</h1>
    <form>
        <textarea name="editor"></textarea>
    </form>
    <script>
        ClassicEditor
            .create( document.querySelector( 'textarea[name="editor"]' ), {
                
                ckfinder: {
                    uploadUrl: '/ckfinder/core/connector/php/connector.php?command=QuickUpload&type=Images&responseType=json',
                },
            } )
            .catch( error => {
                console.error( error );
            } );
    </script>
    <!-- {{phpinfo()}} -->
</body>
</html>