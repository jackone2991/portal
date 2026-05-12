
var loadIMGPreview = function (event) {
    var output = document.getElementById('previewAvatar');
    output.src = URL.createObjectURL(event.target.files[0]);
    output.onload = function () {
        URL.revokeObjectURL(output.src) // free memory
    }
};
$("#btnSaveBook").click(function () {

    
    var file_data = $('#book_avartar').prop('files')[0];
    var form_data = new FormData();
    form_data.append('name_vn', $("#name_vn").val());
    form_data.append('name_en', $("#name_en").val());
    form_data.append('book_type', $("#book_type").val());
    form_data.append('book_cate', $("#book_cate").val());
    form_data.append('book_author', $("#book_author").val());
    form_data.append('file', file_data);


    $.ajax({
        headers: {
            'X-CSRF-TOKEN': $('meta[name="csrf-token"]').attr('content')
        },
        type: "POST",
        url: $("#apiPostSaveBook").val(),
        processData: false,
        contentType: false,
        data: form_data,
    })
        .done(function (data) {
            if (true == data['iSaved']) {
                $("#btnClose").trigger("click");
            }
        })
        .fail(function (data) {
            console.log(data);
            alert("Da co loi xay ra.");
        });
});