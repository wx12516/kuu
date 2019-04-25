package mongo

import (
	"errors"
	"math"
	"reflect"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/kuuland/kuu"
)

const (
	// ALL 全量模式
	ALL = "ALL"
	// PAGE 分页模式
	PAGE       = "PAGE"
	timeFormat = "2006-01-02 15:04:05"
)

// Params 定义了查询参数常用结构
type Params struct {
	ID      string
	Page    int
	Size    int
	Range   string
	Sort    []string
	Project map[string]int
	Cond    kuu.H
}

// Model 基于Mongo的模型操作实现
type Model struct {
	schema     *kuu.Schema
	Scope      *Scope
	Collection string
	Name       string
	Session    *mgo.Session
}

// New 实现New接口
func (m *Model) New(schema *kuu.Schema) kuu.IModel {
	n := &Model{
		schema:     schema,
		Name:       schema.Name,
		Collection: schema.Collection,
	}
	return n
}

// Schema 实现Schema接口
func (m *Model) Schema() *kuu.Schema {
	return m.schema
}

// Create 实现新增（支持传入单个或者数组）
func (m *Model) Create(data interface{}) ([]interface{}, error) {
	now := time.Now()
	m.Scope = &Scope{
		Operation: "Create",
		Cache:     &kuu.H{},
	}
	docs := []interface{}{}
	if kuu.IsArray(data) {
		kuu.JSONConvert(data, &docs)
	} else {
		doc := kuu.H{}
		kuu.JSONConvert(data, &doc)
		docs = append(docs, doc)
	}
	for index, item := range docs {
		var doc kuu.H
		kuu.JSONConvert(item, &doc)
		doc["CreatedAt"] = now.Unix()
		doc["CreatedAtFmt"] = now.Format(timeFormat)
		if doc["CreatedBy"] != nil {
			switch doc["CreatedBy"].(type) {
			case string:
				doc["CreatedBy"] = bson.ObjectIdHex(doc["CreatedBy"].(string))
			case bson.ObjectId:
				doc["CreatedBy"] = doc["CreatedBy"].(bson.ObjectId)
			}
		}
		// 设置UpdatedXx初始值等于CreatedXx
		doc["UpdatedAt"] = doc["CreatedAt"]
		doc["UpdatedAtFmt"] = doc["CreatedAtFmt"]
		doc["UpdatedBy"] = doc["CreatedBy"]
		docs[index] = doc
	}
	C := C(m.Collection)
	m.Session = C.Database.Session
	m.Scope.Session = m.Session
	m.Scope.Collection = C
	defer func() {
		C.Database.Session.Close()
		m.Session = nil
		m.Scope = nil
	}()
	pts := make([]interface{}, 0)
	for _, item := range docs {
		pts = append(pts, &item)
	}
	err := C.Insert(pts...)
	m.Scope.CreateData = &pts
	m.Scope.CallMethod(BeforeSaveEnum, m.schema)
	m.Scope.CallMethod(BeforeCreateEnum, m.schema)
	// 先保存外键
	docs = handleJoinBeforeSave(docs, m.schema)
	m.Scope.CallMethod(AfterCreateEnum, m.schema)
	m.Scope.CallMethod(AfterSaveEnum, m.schema)
	return pts, err
}

// Remove 实现基于条件的逻辑删除
func (m *Model) Remove(selector interface{}) error {
	return m.RemoveWithData(selector, nil)
}

// RemoveWithData 实现基于条件的逻辑删除
func (m *Model) RemoveWithData(selector interface{}, data interface{}) error {
	var (
		cond kuu.H
		doc  kuu.H
	)
	kuu.JSONConvert(selector, &cond)
	kuu.JSONConvert(data, &doc)
	_, err := m.remove(cond, doc, false)
	return err
}

// RemoveEntity 实现基于实体的逻辑删除
func (m *Model) RemoveEntity(entity interface{}) error {
	return m.RemoveEntityWithData(entity, nil)
}

// RemoveEntityWithData 实现基于实体的逻辑删除
func (m *Model) RemoveEntityWithData(entity interface{}, data interface{}) error {
	var (
		doc kuu.H
		obj kuu.H
	)
	kuu.JSONConvert(entity, &obj)
	kuu.JSONConvert(data, &doc)
	if obj == nil || obj["_id"] == nil {
		return errors.New("_id is required")
	}
	cond := kuu.H{
		"_id": obj["_id"],
	}
	_, err := m.remove(cond, doc, false)
	return err
}

// RemoveAll 实现基于条件的批量逻辑删除
func (m *Model) RemoveAll(selector interface{}) (interface{}, error) {
	return m.RemoveAllWithData(selector, nil)
}

// RemoveAllWithData 实现基于条件的批量逻辑删除
func (m *Model) RemoveAllWithData(selector interface{}, data interface{}) (interface{}, error) {
	var (
		cond kuu.H
		doc  kuu.H
	)
	kuu.JSONConvert(selector, &cond)
	kuu.JSONConvert(data, &doc)
	return m.remove(cond, doc, true)
}

// PhyRemove 实现基于条件的物理删除
func (m *Model) PhyRemove(selector interface{}) error {
	var cond kuu.H
	kuu.JSONConvert(selector, &cond)
	_, err := m.phyRemove(cond, false)
	return err
}

// PhyRemoveEntity 实现基于实体的物理删除
func (m *Model) PhyRemoveEntity(entity interface{}) error {
	var obj kuu.H
	kuu.JSONConvert(entity, &obj)
	if obj == nil || obj["_id"] == nil {
		return errors.New("_id is required")
	}
	cond := kuu.H{
		"_id": obj["_id"],
	}
	_, err := m.phyRemove(cond, false)
	return err
}

// PhyRemoveAll 实现基于条件的批量物理删除
func (m *Model) PhyRemoveAll(selector interface{}) (interface{}, error) {
	var cond kuu.H
	kuu.JSONConvert(selector, &cond)
	return m.phyRemove(cond, true)
}

// Update 实现基于条件的更新
func (m *Model) Update(selector interface{}, data interface{}) error {
	var (
		cond kuu.H
		doc  kuu.H
	)
	kuu.JSONConvert(selector, &cond)
	kuu.JSONConvert(data, &doc)
	_, err := m.update(cond, doc, false)
	return err
}

// UpdateEntity 实现基于实体的更新
func (m *Model) UpdateEntity(entity interface{}) error {
	var doc kuu.H
	kuu.JSONConvert(entity, &doc)
	if doc == nil || doc["_id"] == nil {
		return errors.New("_id is required")
	}
	cond := kuu.H{
		"_id": doc["_id"],
	}
	delete(doc, "_id")
	_, err := m.update(cond, doc, false)
	return err
}

// UpdateAll 实现基于条件的批量更新
func (m *Model) UpdateAll(selector interface{}, data interface{}) (interface{}, error) {
	var (
		cond kuu.H
		doc  kuu.H
	)
	kuu.JSONConvert(selector, &cond)
	kuu.JSONConvert(data, &doc)
	return m.update(cond, doc, true)
}

// List 实现列表查询
func (m *Model) List(a interface{}, list interface{}) (kuu.H, error) {
	m.Scope = &Scope{
		Operation: "List",
		Cache:     &kuu.H{},
	}
	p := &Params{}
	kuu.JSONConvert(a, p)
	// 参数加工
	if list == nil {
		list = make([]kuu.H, 0)
	}
	isDeleted := kuu.H{
		"$ne": true,
	}
	if p.Cond == nil {
		p.Cond = make(kuu.H)
	}
	if p.Cond["_id"] != nil {
		if v, ok := p.Cond["_id"].(string); ok {
			p.Cond["_id"] = bson.ObjectIdHex(v)
		} else {
			rv := reflect.ValueOf(p.Cond["_id"])
			switch rv.Kind() {
			case reflect.Map:
				_id := p.Cond["_id"].(map[string]interface{})
				if _, ok := _id["$in"]; ok {
					oldArr := _id["$in"].([]interface{})
					newArr := make([]bson.ObjectId, 0)
					for _, item := range oldArr {
						if v, ok := item.(string); ok {
							newArr = append(newArr, bson.ObjectIdHex(v))
						} else if v, ok := item.(bson.ObjectId); ok {
							newArr = append(newArr, v)
						}
					}
					_id["$in"] = newArr
				}
				p.Cond["_id"] = _id
			}
		}
	}
	if p.Cond["$and"] != nil {
		var and []kuu.H
		kuu.JSONConvert(p.Cond["$and"], &and)
		hasDr := false
		for _, item := range and {
			if item["IsDeleted"] != nil {
				hasDr = true
				break
			}
		}
		if !hasDr {
			and = append(and, kuu.H{
				"IsDeleted": isDeleted,
			})
			p.Cond["$and"] = and
		}
	} else {
		if p.Cond["IsDeleted"] == nil {
			p.Cond["IsDeleted"] = isDeleted
		}
	}

	C := C(m.Collection)
	m.Session = C.Database.Session
	m.Scope.Session = m.Session
	m.Scope.Collection = C
	defer func() {
		C.Database.Session.Close()
		m.Session = nil
		m.Scope = nil
	}()
	p.Cond = handleJoinBeforeQuery(p.Cond, m.schema)
	m.Scope.Params = p
	m.Scope.CallMethod(BeforeFindEnum, m.schema)
	query := C.Find(p.Cond)
	totalRecords, err := query.Count()
	if err != nil {
		return nil, err
	}
	if p.Project != nil {
		query.Select(p.Project)
	}
	if p.Range == PAGE {
		query.Skip((p.Page - 1) * p.Size).Limit(p.Size)
	}
	if p.Sort != nil && len(p.Sort) > 0 {
		query.Sort(p.Sort...)
	}
	var result []kuu.H
	if err := query.All(&result); err != nil {
		return nil, err
	}
	listJoin(m.Scope.Session, m.schema, p.Project, result)
	kuu.JSONConvert(result, list)
	if list == nil {
		list = make([]kuu.H, 0)
	}
	data := kuu.H{
		"list":         list,
		"totalrecords": totalRecords,
	}
	if p.Range == PAGE {
		totalpages := int(math.Ceil(float64(totalRecords) / float64(p.Size)))
		data["totalpages"] = totalpages
		data["page"] = p.Page
		data["size"] = p.Size
	}
	if p.Sort != nil && len(p.Sort) > 0 {
		data["sort"] = p.Sort
	}
	if p.Project != nil {
		data["project"] = p.Project
	}
	if p.Cond != nil {
		data["cond"] = p.Cond
	}
	if p.Range != "" {
		data["range"] = p.Range
	}
	m.Scope.ListData = data
	m.Scope.CallMethod(AfterFindEnum, m.schema)
	data = m.Scope.ListData
	return data, nil
}

// ID 实现ID查询（支持传入“string”、“bson.ObjectId”和“&mongo.Params”三种类型的数据）
func (m *Model) ID(id interface{}, data interface{}) error {
	m.Scope = &Scope{
		Operation: "ID",
		Cache:     &kuu.H{},
	}
	p := &Params{}
	switch id.(type) {
	case Params:
		p = id.(*Params)
	case bson.ObjectId:
		p = &Params{
			ID: id.(bson.ObjectId).Hex(),
		}
	case string:
		p = &Params{
			ID: id.(string),
		}
	}
	if p.Cond == nil {
		p.Cond = make(kuu.H)
	}
	if p.ID == "" {
		kuu.JSONConvert(id, p)
	}
	C := C(m.Collection)
	m.Session = C.Database.Session
	m.Scope.Session = m.Session
	m.Scope.Collection = C
	defer func() {
		C.Database.Session.Close()
		m.Session = nil
		m.Scope = nil
	}()
	m.Scope.Params = p
	m.Scope.CallMethod(BeforeFindEnum, m.schema)
	v := p.ID
	query := C.FindId(bson.ObjectIdHex(v))
	if p.Project != nil {
		query.Select(p.Project)
	}
	result := kuu.H{}
	err := query.One(&result)
	if err == nil {
		oneJoin(m.Scope.Session, m.schema, p.Project, result)
		kuu.JSONConvert(&result, data)
	}
	m.Scope.CallMethod(AfterFindEnum, m.schema)
	return err
}

// One 实现单个查询
func (m *Model) One(a interface{}, data interface{}) error {
	m.Scope = &Scope{
		Operation: "One",
		Cache:     &kuu.H{},
	}
	p := &Params{}
	kuu.JSONConvert(a, p)
	if p.Cond == nil {
		p.Cond = make(kuu.H)
	}
	C := C(m.Collection)
	m.Session = C.Database.Session
	m.Scope.Session = m.Session
	m.Scope.Collection = C
	defer func() {
		C.Database.Session.Close()
		m.Session = nil
		m.Scope = nil
	}()
	m.Scope.Params = p
	m.Scope.CallMethod(BeforeFindEnum, m.schema)
	query := C.Find(p.Cond)
	if p.Project != nil {
		query.Select(p.Project)
	}
	if p.Sort != nil && len(p.Sort) > 0 {
		query.Sort(p.Sort...)
	}
	result := kuu.H{}
	err := query.One(&result)
	if err == nil {
		oneJoin(m.Scope.Session, m.schema, p.Project, result)
		kuu.JSONConvert(&result, data)
	}
	m.Scope.CallMethod(AfterFindEnum, m.schema)
	return err
}

func (m *Model) remove(cond kuu.H, doc kuu.H, all bool) (ret interface{}, err error) {
	now := time.Now()
	m.Scope = &Scope{
		Operation: "Remove",
		Cache:     &kuu.H{},
	}
	C := C(m.Collection)
	m.Session = C.Database.Session
	m.Scope.Session = m.Session
	m.Scope.Collection = C
	defer func() {
		C.Database.Session.Close()
		m.Session = nil
		m.Scope = nil
	}()
	if doc == nil {
		doc = make(kuu.H)
	}
	// 确保_id参数为ObjectId类型
	cond = checkID(cond)
	doc = checkUpdateSet(doc)
	_set := doc["$set"].(kuu.H)
	// 删除不合法字段
	delete(_set, "_id")
	delete(_set, "CreatedAt")
	delete(_set, "CreatedBy")
	_set["IsDeleted"] = true
	_set["UpdatedAt"] = now.Unix()
	_set["UpdatedAtFmt"] = now.Format(timeFormat)
	if _set["UpdatedBy"] != nil {
		switch _set["UpdatedBy"].(type) {
		case string:
			_set["UpdatedBy"] = bson.ObjectIdHex(_set["UpdatedBy"].(string))
		case bson.ObjectId:
			_set["UpdatedBy"] = _set["UpdatedBy"].(bson.ObjectId)
		}
	}
	doc["$set"] = _set
	m.Scope.CallMethod(BeforeRemoveEnum, m.schema)
	if all {
		ret, err = C.UpdateAll(cond, doc)
	}
	// 外键数据检测
	rets := handleJoinBeforeSave([]interface{}{doc["$set"]}, m.schema)
	_set = rets[0].(kuu.H)
	doc["$set"] = _set
	err = C.Update(cond, doc)
	m.Scope.CallMethod(AfterRemoveEnum, m.schema)
	return ret, err
}

func (m *Model) phyRemove(cond kuu.H, all bool) (ret interface{}, err error) {
	m.Scope = &Scope{
		Operation: "PhyRemove",
		Cache:     &kuu.H{},
	}
	C := C(m.Collection)
	m.Session = C.Database.Session
	m.Scope.Session = m.Session
	m.Scope.Collection = C
	defer func() {
		C.Database.Session.Close()
		m.Session = nil
		m.Scope = nil
	}()
	cond = checkID(cond)
	m.Scope.CallMethod(BeforePhyRemoveEnum, m.schema)
	if all {
		ret, err = C.RemoveAll(cond)
	}
	err = C.Remove(cond)
	m.Scope.CallMethod(AfterPhyRemoveEnum, m.schema)
	return ret, err
}

func (m *Model) update(cond kuu.H, doc kuu.H, all bool) (ret interface{}, err error) {
	now := time.Now()
	m.Scope = &Scope{
		Operation: "Update",
		Cache:     &kuu.H{},
	}
	C := C(m.Collection)
	m.Session = C.Database.Session
	m.Scope.Session = m.Session
	m.Scope.Collection = C
	defer func() {
		C.Database.Session.Close()
		m.Session = nil
		m.Scope = nil
	}()
	cond = checkID(cond)
	doc = checkUpdateSet(doc)
	_set := doc["$set"].(kuu.H)
	// 删除不合法字段
	delete(_set, "_id")
	delete(_set, "CreatedAt")
	delete(_set, "CreatedBy")
	_set["UpdatedAt"] = now.Unix()
	_set["UpdatedAtFmt"] = now.Format(timeFormat)
	if _set["UpdatedBy"] != nil {
		switch _set["UpdatedBy"].(type) {
		case string:
			_set["UpdatedBy"] = bson.ObjectIdHex(_set["UpdatedBy"].(string))
		case bson.ObjectId:
			_set["UpdatedBy"] = _set["UpdatedBy"].(bson.ObjectId)
		}
	}
	doc["$set"] = _set

	m.Scope.UpdateCond = &cond
	m.Scope.UpdateDoc = &doc
	m.Scope.CallMethod(BeforeSaveEnum, m.schema)
	m.Scope.CallMethod(BeforeUpdateEnum, m.schema)
	if all {
		ret, err = C.UpdateAll(cond, doc)
	}
	// 外键数据检测
	rets := handleJoinBeforeSave([]interface{}{doc["$set"]}, m.schema)
	_set = rets[0].(kuu.H)
	doc["$set"] = _set
	err = C.Update(cond, doc)
	m.Scope.CallMethod(AfterUpdateEnum, m.schema)
	m.Scope.CallMethod(AfterSaveEnum, m.schema)
	return ret, err
}

func checkID(cond kuu.H) kuu.H {
	if v, ok := cond["_id"].(string); ok {
		cond["_id"] = bson.ObjectIdHex(v)
	} else if v, ok := cond["_id"].(bson.ObjectId); ok {
		cond["_id"] = v
	} else {
		var v kuu.H
		kuu.JSONConvert(cond["_id"], &v)
		if v["$in"] != nil {
			arr := []bson.ObjectId{}
			if vv, ok := v["$in"].([]interface{}); ok {
				for _, str := range vv {
					var a bson.ObjectId
					if v, ok := str.(string); ok && v != "" {
						a = bson.ObjectIdHex(v)
					} else if v, ok := str.(bson.ObjectId); ok && v != "" {
						a = v
					}
					if a != "" {
						arr = append(arr, a)
					}
				}
			} else if vv, ok := v["$in"].([]string); ok {
				for _, str := range vv {
					arr = append(arr, bson.ObjectIdHex(str))
				}
			}
			v["$in"] = arr
			cond["_id"] = v
		}
	}
	return cond
}

func checkUpdateSet(doc kuu.H) kuu.H {
	if doc["$set"] == nil {
		doc = kuu.H{
			"$set": doc,
		}
	}
	return doc
}
