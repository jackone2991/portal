<!-- Modal -->
<div class="modal fade" id="create-photo-album" tabindex="-1" role="dialog" aria-labelledby="exampleModalLabel" aria-hidden="true">
  <div class="modal-dialog" role="document">
    <div class="modal-content">
      <div class="modal-header">
        <h5 class="modal-title" id="exampleModalLabel">{{__('message.Add')}}</h5>
        <!-- <button type="button" class="close" data-bs-dismiss="modal" aria-label="Close">
          <span aria-hidden="true">x</span>
        </button> -->
      </div>
      <div class="modal-body">
    <form class="content" data-bitwarden-watching="1" id="bookForm">
        <div class="row">
            <div class="col col-12 col-xl-12 col-lg-12 col-md-12 col-sm-12">
                <div class="form-group label-floating is-empty">
                    <label class="control-label">{{__('message.name_en')}}</label>
                    <input type="hidden" value="{{ csrf_token() }}" id="csrf_token">
                    <input class="form-control" placeholder="" type="input" name="name_en" id="name_en">
                    <span class="material-input"></span>
                </div>
                <div class="form-group label-floating is-empty">
                    <label class="control-label">{{__('message.name_vn')}}</label>
                    <input class="form-control" placeholder="" type="input" name="name_vn" id="name_vn">
                    <span class="material-input"></span>
                </div>
                <div class="form-group label-floating is-select">
                    <label class="control-label">{{__('message.book_type')}}</label>
                    <select class="form-select form-control" name="book_type" id="book_type">
                      @foreach($allLibType as $item)
                        @if('vn' == Config::get('app.locale'))
                          <option value="{{$item['id']}}">{{$item['name_vn']}}</option>
                        @else
                          <option value="{{$item['id']}}">{{$item['name_en']}}</option>
                        @endif
                      @endforeach
                    </select>
                    <span class="material-input"></span>
                </div>
                <div class="form-group label-floating is-select">
                    <label class="control-label">{{__('message.book_cate')}}</label>
                    <select class="form-select form-control" name="book_cate" id="book_cate">
                      @foreach($allLibCate as $item)
                        @if('vn' == Config::get('app.locale'))
                          <option value="{{$item['id']}}">{{$item['name_vn']}}</option>
                        @else
                          <option value="{{$item['id']}}">{{$item['name_en']}}</option>
                        @endif
                      @endforeach
                    </select>
                    <span class="material-input"></span>
                </div>
                
                <div class="form-group label-floating is-empty">
                    <label class="control-label">{{__('message.book_author')}}</label>
                    <input class="form-control" placeholder="" type="input" name="book_author" id="book_author">
                    <span class="material-input"></span>
                </div>
                <div class="form-group label-floating is-empty" >
                    <label for="book_avartar" class="btn btn-info btn-primary" style="width:100%">{{__('message.book_avartar')}}</label>
                    <input class="form-control" id="book_avartar" placeholder="Avatar" type="file" style="display:none"  onchange="loadIMGPreview(event)" name="book_avartar">
                </div>
                <div width="100" height="100">
                  <img id="previewAvatar" alt="your image" width="100" height="100" />
                </div>
                 <!-- <div class="form-group label-floating is-empty">
                    <label class="control-label">Status</label>
                    <div class="togglebutton">
                        <label>
                            <input type="checkbox" checked=""><span class="toggle"></span>
                        </label>
                    </div>
                </div> -->
            </div>
        </div>
    </form>
<script>

    // });
</script>



























      </div>
      <div class="modal-footer">
        <button id="btnClose" type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{__('message.close')}}</button>
        <input type="hidden" value="{{route('api.post.savebook')}}" id="apiPostSaveBook">
        <button id="btnSaveBook" type="submit" class="btn btn-primary">{{__('message.save')}}</button>
      </div>
    </div>
  </div>
</div>